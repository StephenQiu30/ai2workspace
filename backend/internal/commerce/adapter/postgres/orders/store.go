package orders

import (
	"context"
	"errors"
	"fmt"
	"time"

	ordersapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/orders"
	ordersdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/orders"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store backed by PostgreSQL via pgxpool.
type PostgresStore struct {
	pool               *pgxpool.Pool
	saleVerifier       ordersapp.SaleSnapshotVerifier
	entitlementChecker ordersapp.ActiveEntitlementChecker
}

// NewPostgresStore creates a PostgreSQL-backed orders store.
func NewPostgresStore(pool *pgxpool.Pool, verifier ordersapp.SaleSnapshotVerifier, checker ordersapp.ActiveEntitlementChecker) *PostgresStore {
	return &PostgresStore{
		pool:               pool,
		saleVerifier:       verifier,
		entitlementChecker: checker,
	}
}

func (s *PostgresStore) SetCoursePrice(ctx context.Context, courseID string, amountMinor int64, currency string, expectedVersion int) (*ordersdomain.Price, error) {
	if amountMinor < 0 {
		return nil, ordersdomain.ErrInvalidAmount
	}
	cur, err := ordersapp.ValidateCurrency(currency)
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
	var price ordersdomain.Price
	if errors.Is(err, pgx.ErrNoRows) {
		if expectedVersion > 0 {
			return nil, ordersdomain.ErrVersionConflict
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
			return nil, ordersdomain.ErrVersionConflict
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

func (s *PostgresStore) GetCoursePrice(ctx context.Context, courseID string) (*ordersdomain.Price, error) {
	var p ordersdomain.Price
	err := s.pool.QueryRow(ctx, `
		SELECT course_id, amount_minor, currency, version, updated_at
		FROM prices WHERE course_id = $1
	`, courseID).Scan(&p.CourseID, &p.AmountMinor, &p.Currency, &p.Version, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ordersdomain.ErrPriceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *PostgresStore) CreateOrder(ctx context.Context, userID, courseID, idempotencyKey string) (*ordersdomain.Order, error) {
	// 1. Check idempotency first.
	existing, err := s.getByIdempotency(ctx, userID, idempotencyKey)
	if err == nil {
		if existing.CourseID != courseID {
			return nil, ordersdomain.ErrIdempotencyMismatch
		}
		return existing, nil
	} else if !errors.Is(err, ordersdomain.ErrOrderNotFound) {
		return nil, err
	}

	// 2. Verify course sale snapshot from learning before opening DB transaction.
	var courseTitle string
	if s.saleVerifier != nil {
		snap, err := s.saleVerifier(ctx, courseID)
		if err != nil {
			if errors.Is(err, ordersdomain.ErrCourseNotFound) {
				return nil, ordersdomain.ErrCourseNotFound
			}
			return nil, fmt.Errorf("%w: %v", ordersdomain.ErrUpstreamUnavailable, err)
		}
		if snap == nil {
			return nil, ordersdomain.ErrUpstreamUnavailable
		}
		if !ordersapp.IsOnSale(snap.SaleState) {
			return nil, ordersdomain.ErrCourseNotForSale
		}
		if snap.IsFree {
			return nil, ordersdomain.ErrFreeCourseOrderNotAllowed
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
			return nil, ordersdomain.ErrAlreadyEntitled
		}
	}

	// 4. Read authoritative price and insert order.
	price, err := s.GetCoursePrice(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if price.AmountMinor <= 0 {
		return nil, ordersdomain.ErrFreeCourseOrderNotAllowed
	}

	now := time.Now().UTC()
	orderID := ordersapp.NewID("ord")
	var ord ordersdomain.Order
	var statusStr string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO orders (id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'v1', $9, $9)
		ON CONFLICT (user_id, idempotency_key) DO NOTHING
		RETURNING id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
	`, orderID, userID, courseID, courseTitle, price.AmountMinor, price.Currency, string(ordersdomain.StatusPendingPayment), idempotencyKey, now).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, lookupErr := s.getByIdempotency(ctx, userID, idempotencyKey)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing.CourseID != courseID {
			return nil, ordersdomain.ErrIdempotencyMismatch
		}
		return existing, nil
	}
	if err != nil {
		return nil, err
	}
	ord.Status = ordersdomain.OrderStatus(statusStr)
	return &ord, nil
}

func (s *PostgresStore) getByIdempotency(ctx context.Context, userID, idempotencyKey string) (*ordersdomain.Order, error) {
	var ord ordersdomain.Order
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
		FROM orders WHERE user_id = $1 AND idempotency_key = $2
	`, userID, idempotencyKey).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ordersdomain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	ord.Status = ordersdomain.OrderStatus(statusStr)
	return &ord, nil
}

func (s *PostgresStore) GetOrder(ctx context.Context, orderID string) (*ordersdomain.Order, error) {
	var ord ordersdomain.Order
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
		FROM orders WHERE id = $1
	`, orderID).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ordersdomain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	ord.Status = ordersdomain.OrderStatus(statusStr)
	return &ord, nil
}

func (s *PostgresStore) ListOrders(ctx context.Context, userID string, page, limit int) ([]*ordersdomain.Order, int, error) {
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

	list := make([]*ordersdomain.Order, 0)
	for rows.Next() {
		var ord ordersdomain.Order
		var statusStr string
		if err := rows.Scan(
			&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
			&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		ord.Status = ordersdomain.OrderStatus(statusStr)
		list = append(list, &ord)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *PostgresStore) CancelOrder(ctx context.Context, userID, orderID string) (*ordersdomain.Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID, statusStr string
	err = tx.QueryRow(ctx, `SELECT user_id, status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&ownerID, &statusStr)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ordersdomain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if ownerID != userID {
		return nil, ordersdomain.ErrPermissionDenied
	}
	if ordersdomain.OrderStatus(statusStr) != ordersdomain.StatusPendingPayment {
		return nil, ordersdomain.ErrOrderNotPending
	}

	var ord ordersdomain.Order
	err = tx.QueryRow(ctx, `
		UPDATE orders
		SET status = $2, updated_at = $3
		WHERE id = $1
		RETURNING id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
	`, orderID, string(ordersdomain.StatusCancelled), time.Now().UTC()).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	ord.Status = ordersdomain.OrderStatus(statusStr)
	return &ord, nil
}

func (s *PostgresStore) MarkOrderPaid(ctx context.Context, orderID string, paidAt time.Time) (*ordersdomain.Order, error) {
	var ord ordersdomain.Order
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		UPDATE orders
		SET status = $2, paid_at = $3, updated_at = $4
		WHERE id = $1 AND status = $5
		RETURNING id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
	`, orderID, string(ordersdomain.StatusPaid), paidAt.UTC(), time.Now().UTC(), string(ordersdomain.StatusPendingPayment)).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, lookupErr := s.GetOrder(ctx, orderID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing.Status != ordersdomain.StatusPendingPayment {
			return nil, ordersdomain.ErrOrderNotPending
		}
		return nil, ordersdomain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	ord.Status = ordersdomain.OrderStatus(statusStr)
	return &ord, nil
}

func (s *PostgresStore) MarkOrderRefunded(ctx context.Context, orderID string) (*ordersdomain.Order, error) {
	var ord ordersdomain.Order
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		UPDATE orders
		SET status = $2, updated_at = $3
		WHERE id = $1 AND status = $4
		RETURNING id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
	`, orderID, string(ordersdomain.StatusRefunded), time.Now().UTC(), string(ordersdomain.StatusPaid)).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, lookupErr := s.GetOrder(ctx, orderID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing.Status == ordersdomain.StatusRefunded {
			return existing, nil
		}
		return nil, ordersdomain.ErrOrderNotPaid
	}
	if err != nil {
		return nil, err
	}
	ord.Status = ordersdomain.OrderStatus(statusStr)
	return &ord, nil
}
