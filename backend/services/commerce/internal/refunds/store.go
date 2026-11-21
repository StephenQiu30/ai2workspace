package refunds

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/entitlements"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store backed by PostgreSQL via pgxpool.
type PostgresStore struct {
	pool             *pgxpool.Pool
	orderStore       orders.Store
	entitlementStore entitlements.Store
	evidenceProvider DeliveryEvidenceProvider
	notifier         Notifier
}

// NewPostgresStore creates a PostgreSQL-backed refunds store.
func NewPostgresStore(pool *pgxpool.Pool, orderStore orders.Store, entStore entitlements.Store, evidenceProvider DeliveryEvidenceProvider, notifier Notifier) *PostgresStore {
	return &PostgresStore{
		pool:             pool,
		orderStore:       orderStore,
		entitlementStore: entStore,
		evidenceProvider: evidenceProvider,
		notifier:         notifier,
	}
}

func scanRefundRow(row pgx.Row) (*RefundApplication, error) {
	var app RefundApplication
	var appStateStr, chStateStr string
	err := row.Scan(
		&app.ID, &app.OrderID, &app.UserID, &app.Reason, &app.ExceptionType, &app.IsOutOfWindow,
		&app.AmountMinor, &appStateStr, &chStateStr, &app.ChannelRefundID, &app.ChannelFailureReason,
		&app.ReviewerID, &app.ReviewedAt, &app.RejectionReason, &app.Version, &app.FirstAvailableAt,
		&app.Deadline, &app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	app.ApplicationState = ApplicationState(appStateStr)
	app.ChannelState = ChannelState(chStateStr)
	return &app, nil
}

const refundSelectCols = `
	id, order_id, user_id, reason, exception_type, is_out_of_window,
	amount_minor, application_state, channel_state, channel_refund_id, channel_failure_reason,
	reviewer_id, reviewed_at, rejection_reason, version, first_available_at,
	deadline, created_at, updated_at
`

func (s *PostgresStore) ApplyRefund(ctx context.Context, userID, orderID, reason, exceptionType string) (*RefundApplication, error) {
	reason = strings.TrimSpace(reason)
	exceptionType = strings.TrimSpace(exceptionType)

	if exceptionType != "" && !IsValidExceptionType(exceptionType) {
		return nil, ErrInvalidExceptionType
	}

	order, err := s.orderStore.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, ErrPermissionDenied
	}
	if order.Status != orders.StatusPaid {
		return nil, ErrOrderNotEligible
	}

	now := time.Now().UTC()
	firstAvailableAt, deadline, isOutOfWindow, err := computeRefundWindow(ctx, order, s.evidenceProvider, now)
	if err != nil {
		return nil, err
	}
	if isOutOfWindow && exceptionType == "" {
		return nil, ErrRefundWindowExpired
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock order row to serialize concurrent refund applications for the same order.
	var ordStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&ordStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, orders.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if orders.OrderStatus(ordStatus) != orders.StatusPaid {
		return nil, ErrOrderNotEligible
	}

	existing, err := scanRefundRow(tx.QueryRow(ctx, `
		SELECT `+refundSelectCols+`
		FROM refund_applications
		WHERE order_id = $1 AND application_state IN ('pending_review', 'approved')
		ORDER BY created_at DESC
		LIMIT 1
	`, orderID))
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	refID := generateID("ref")
	app, err := scanRefundRow(tx.QueryRow(ctx, `
		INSERT INTO refund_applications (
			id, order_id, user_id, reason, exception_type, is_out_of_window,
			amount_minor, application_state, channel_state, version,
			first_available_at, deadline, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1, $10, $11, $12, $12)
		RETURNING `+refundSelectCols,
		refID, orderID, userID, reason, exceptionType, isOutOfWindow,
		order.AmountMinor, string(StatePendingReview), string(ChannelNone),
		firstAvailableAt, deadline, now,
	))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return app, nil
}

func (s *PostgresStore) CancelRefund(ctx context.Context, userID, applicationID string, expectedVersion int) (*RefundApplication, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	app, err := scanRefundRow(tx.QueryRow(ctx, `SELECT `+refundSelectCols+` FROM refund_applications WHERE id = $1 FOR UPDATE`, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefundNotFound
	}
	if err != nil {
		return nil, err
	}
	if app.UserID != userID {
		return nil, ErrPermissionDenied
	}
	if app.ApplicationState != StatePendingReview {
		return nil, ErrRefundNotPending
	}
	if expectedVersion > 0 && app.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	updated, err := scanRefundRow(tx.QueryRow(ctx, `
		UPDATE refund_applications
		SET application_state = $2, version = version + 1, updated_at = $3
		WHERE id = $1
		RETURNING `+refundSelectCols,
		applicationID, string(StateCancelled), time.Now().UTC(),
	))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *PostgresStore) ReviewRefund(ctx context.Context, adminID, applicationID, decision, rejectionReason string, expectedVersion int) (*RefundApplication, error) {
	if decision != "approved" && decision != "rejected" {
		return nil, ErrInvalidReviewDecision
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	app, err := scanRefundRow(tx.QueryRow(ctx, `SELECT `+refundSelectCols+` FROM refund_applications WHERE id = $1 FOR UPDATE`, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefundNotFound
	}
	if err != nil {
		return nil, err
	}
	if app.ApplicationState != StatePendingReview {
		return nil, ErrRefundNotPending
	}
	if expectedVersion > 0 && app.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	now := time.Now().UTC()
	nextAppState := StateRejected
	nextChState := ChannelNone
	rejReason := strings.TrimSpace(rejectionReason)
	if decision == "approved" {
		nextAppState = StateApproved
		nextChState = ChannelSubmitting
		rejReason = ""
	}

	updated, err := scanRefundRow(tx.QueryRow(ctx, `
		UPDATE refund_applications
		SET application_state = $2,
		    channel_state = $3,
		    reviewer_id = $4,
		    reviewed_at = $5,
		    rejection_reason = $6,
		    version = version + 1,
		    updated_at = $5
		WHERE id = $1
		RETURNING `+refundSelectCols,
		applicationID, string(nextAppState), string(nextChState), adminID, now, rejReason,
	))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *PostgresStore) ConfirmChannelRefund(ctx context.Context, applicationID string, status ChannelState, channelRefundID, failureReason string, expectedVersion int) (*RefundApplication, error) {
	if status != ChannelSuccess && status != ChannelFailed && status != ChannelUnknown {
		return nil, ErrInvalidChannelState
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	app, err := scanRefundRow(tx.QueryRow(ctx, `SELECT `+refundSelectCols+` FROM refund_applications WHERE id = $1 FOR UPDATE`, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefundNotFound
	}
	if err != nil {
		return nil, err
	}
	if app.ApplicationState != StateApproved {
		return nil, ErrRefundNotApproved
	}
	if expectedVersion > 0 && app.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	if app.ChannelState == ChannelSuccess {
		if status == ChannelSuccess {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return app, nil
		}
		return nil, ErrRefundTerminalState
	}

	now := time.Now().UTC()
	chRefID := app.ChannelRefundID
	if channelRefundID != "" {
		chRefID = channelRefundID
	}
	failReason := strings.TrimSpace(failureReason)
	if status == ChannelSuccess {
		failReason = ""
		// Atomically revoke ONLY the entitlement for app.OrderID and mark order refunded.
		_, err = tx.Exec(ctx, `
			UPDATE entitlements
			SET status = 'revoked', revoked_at = $2, grant_version = grant_version + 1
			WHERE order_id = $1 AND status = 'active'
		`, app.OrderID, now)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `
			UPDATE orders
			SET status = 'refunded', updated_at = $2
			WHERE id = $1 AND status = 'paid'
		`, app.OrderID, now)
		if err != nil {
			return nil, err
		}
	}

	updated, err := scanRefundRow(tx.QueryRow(ctx, `
		UPDATE refund_applications
		SET channel_state = $2,
		    channel_refund_id = $3,
		    channel_failure_reason = $4,
		    version = version + 1,
		    updated_at = $5
		WHERE id = $1
		RETURNING `+refundSelectCols,
		applicationID, string(status), chRefID, failReason, now,
	))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if status == ChannelSuccess {
		sendRefundSucceededNotification(ctx, s.notifier, updated)
	}
	return updated, nil
}

func (s *PostgresStore) GetRefund(ctx context.Context, applicationID string) (*RefundApplication, error) {
	app, err := scanRefundRow(s.pool.QueryRow(ctx, `SELECT `+refundSelectCols+` FROM refund_applications WHERE id = $1`, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefundNotFound
	}
	if err != nil {
		return nil, err
	}
	return app, nil
}

func (s *PostgresStore) ListRefunds(ctx context.Context, page, limit int) ([]*RefundApplication, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM refund_applications`).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := s.pool.Query(ctx, `
		SELECT `+refundSelectCols+`
		FROM refund_applications
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*RefundApplication, 0)
	for rows.Next() {
		app, err := scanRefundRow(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, app)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *PostgresStore) ListRefundsByOrder(ctx context.Context, orderID string) ([]*RefundApplication, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+refundSelectCols+`
		FROM refund_applications
		WHERE order_id = $1
		ORDER BY created_at DESC, id DESC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*RefundApplication, 0)
	for rows.Next() {
		app, err := scanRefundRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, app)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
