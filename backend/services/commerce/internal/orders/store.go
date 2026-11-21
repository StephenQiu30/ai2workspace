package orders

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store backed by PostgreSQL via pgxpool.
type PostgresStore struct {
	pool               *pgxpool.Pool
	saleVerifier       SaleSnapshotVerifier
	entitlementChecker ActiveEntitlementChecker
}

// NewPostgresStore creates a PostgreSQL-backed orders store.
func NewPostgresStore(pool *pgxpool.Pool, verifier SaleSnapshotVerifier, checker ActiveEntitlementChecker) *PostgresStore {
	return &PostgresStore{
		pool:               pool,
		saleVerifier:       verifier,
		entitlementChecker: checker,
	}
}

func (s *PostgresStore) SetCoursePrice(ctx context.Context, courseID string, amountMinor int64, currency string, expectedVersion int) (*Price, error) {
	if amountMinor < 0 {
		return nil, ErrInvalidAmount
	}
	cur, err := validateCurrency(currency)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentVersion int
	err = tx.QueryRow(ctx, `SELECT version FROM prices WHERE course_id = $1 FOR UPDATE`, courseID).Scan(&currentVersion)
	now := time.Now().UTC()
	var price Price
	if errors.Is(err, pgx.ErrNoRows) {
		if expectedVersion > 0 {
			return nil, ErrVersionConflict
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO prices (course_id, amount_minor, currency, version, updated_at)
			VALUES ($1, $2, $3, 1, $4)
			RETURNING course_id, amount_minor, currency, version, updated_at
		`, courseID, amountMinor, cur, now).Scan(&price.CourseID, &price.AmountMinor, &price.Currency, &price.Version, &price.UpdatedAt)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		if expectedVersion > 0 && currentVersion != expectedVersion {
			return nil, ErrVersionConflict
		}
		err = tx.QueryRow(ctx, `
			UPDATE prices
			SET amount_minor = $2, currency = $3, version = version + 1, updated_at = $4
			WHERE course_id = $1
			RETURNING course_id, amount_minor, currency, version, updated_at
		`, courseID, amountMinor, cur, now).Scan(&price.CourseID, &price.AmountMinor, &price.Currency, &price.Version, &price.UpdatedAt)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &price, nil
}

func (s *PostgresStore) GetCoursePrice(ctx context.Context, courseID string) (*Price, error) {
	var p Price
	err := s.pool.QueryRow(ctx, `
		SELECT course_id, amount_minor, currency, version, updated_at
		FROM prices WHERE course_id = $1
	`, courseID).Scan(&p.CourseID, &p.AmountMinor, &p.Currency, &p.Version, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPriceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *PostgresStore) CreateOrder(ctx context.Context, userID, courseID, idempotencyKey string) (*Order, error) {
	// 1. Check idempotency first.
	existing, err := s.getByIdempotency(ctx, userID, idempotencyKey)
	if err == nil {
		if existing.CourseID != courseID {
			return nil, ErrIdempotencyMismatch
		}
		return existing, nil
	} else if !errors.Is(err, ErrOrderNotFound) {
		return nil, err
	}

	// 2. Verify course sale snapshot from learning before opening DB transaction.
	var courseTitle string
	if s.saleVerifier != nil {
		snap, err := s.saleVerifier(ctx, courseID)
		if err != nil {
			if errors.Is(err, ErrCourseNotFound) {
				return nil, ErrCourseNotFound
			}
			return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
		}
		if snap == nil {
			return nil, ErrUpstreamUnavailable
		}
		if !isOnSale(snap.SaleState) {
			return nil, ErrCourseNotForSale
		}
		if snap.IsFree {
			return nil, ErrFreeCourseOrderNotAllowed
		}
		courseTitle = snap.Title
	}

	// 3. Check active entitlement.
	if s.entitlementChecker != nil {
		hasEntitlement, err := s.entitlementChecker(ctx, userID, courseID)
		if err != nil {
			return nil, err
		}
		if hasEntitlement {
			return nil, ErrAlreadyEntitled
		}
	}

	// 4. Read authoritative price and insert order.
	price, err := s.GetCoursePrice(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if price.AmountMinor <= 0 {
		return nil, ErrFreeCourseOrderNotAllowed
	}

	now := time.Now().UTC()
	orderID := generateID("ord")
	var ord Order
	var statusStr string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO orders (id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'v1', $9, $9)
		ON CONFLICT (user_id, idempotency_key) DO NOTHING
		RETURNING id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
	`, orderID, userID, courseID, courseTitle, price.AmountMinor, price.Currency, string(StatusPendingPayment), idempotencyKey, now).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, lookupErr := s.getByIdempotency(ctx, userID, idempotencyKey)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing.CourseID != courseID {
			return nil, ErrIdempotencyMismatch
		}
		return existing, nil
	}
	if err != nil {
		return nil, err
	}
	ord.Status = OrderStatus(statusStr)
	return &ord, nil
}

func (s *PostgresStore) getByIdempotency(ctx context.Context, userID, idempotencyKey string) (*Order, error) {
	var ord Order
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
		FROM orders WHERE user_id = $1 AND idempotency_key = $2
	`, userID, idempotencyKey).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	ord.Status = OrderStatus(statusStr)
	return &ord, nil
}

func (s *PostgresStore) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	var ord Order
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
		FROM orders WHERE id = $1
	`, orderID).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	ord.Status = OrderStatus(statusStr)
	return &ord, nil
}

func (s *PostgresStore) ListOrders(ctx context.Context, userID string, page, limit int) ([]*Order, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*Order, 0)
	for rows.Next() {
		var ord Order
		var statusStr string
		if err := rows.Scan(
			&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
			&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		ord.Status = OrderStatus(statusStr)
		list = append(list, &ord)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *PostgresStore) CancelOrder(ctx context.Context, userID, orderID string) (*Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID, statusStr string
	err = tx.QueryRow(ctx, `SELECT user_id, status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&ownerID, &statusStr)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if ownerID != userID {
		return nil, ErrPermissionDenied
	}
	if OrderStatus(statusStr) != StatusPendingPayment {
		return nil, ErrOrderNotPending
	}

	var ord Order
	err = tx.QueryRow(ctx, `
		UPDATE orders
		SET status = $2, updated_at = $3
		WHERE id = $1
		RETURNING id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
	`, orderID, string(StatusCancelled), time.Now().UTC()).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	ord.Status = OrderStatus(statusStr)
	return &ord, nil
}

func (s *PostgresStore) MarkOrderPaid(ctx context.Context, orderID string, paidAt time.Time) (*Order, error) {
	var ord Order
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		UPDATE orders
		SET status = $2, paid_at = $3, updated_at = $4
		WHERE id = $1 AND status = $5
		RETURNING id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
	`, orderID, string(StatusPaid), paidAt.UTC(), time.Now().UTC(), string(StatusPendingPayment)).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, lookupErr := s.GetOrder(ctx, orderID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing.Status != StatusPendingPayment {
			return nil, ErrOrderNotPending
		}
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	ord.Status = OrderStatus(statusStr)
	return &ord, nil
}

func (s *PostgresStore) MarkOrderRefunded(ctx context.Context, orderID string) (*Order, error) {
	var ord Order
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		UPDATE orders
		SET status = $2, updated_at = $3
		WHERE id = $1 AND status = $4
		RETURNING id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
	`, orderID, string(StatusRefunded), time.Now().UTC(), string(StatusPaid)).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, lookupErr := s.GetOrder(ctx, orderID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing.Status == StatusRefunded {
			return existing, nil
		}
		return nil, ErrOrderNotPaid
	}
	if err != nil {
		return nil, err
	}
	ord.Status = OrderStatus(statusStr)
	return &ord, nil
}
