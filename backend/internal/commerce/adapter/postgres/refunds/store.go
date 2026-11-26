package refunds

import (
	"context"
	"errors"
	"strings"
	"time"

	entitlementsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/entitlements"
	ordersapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/orders"
	refundsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/refunds"
	ordersdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/orders"
	refundsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/refunds"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store backed by PostgreSQL via pgxpool.
type PostgresStore struct {
	pool             *pgxpool.Pool
	orderStore       ordersapp.Store
	entitlementStore entitlementsapp.Store
	evidenceProvider refundsapp.DeliveryEvidenceProvider
	notifier         refundsapp.Notifier
}

// NewPostgresStore creates a PostgreSQL-backed refunds store.
func NewPostgresStore(pool *pgxpool.Pool, orderStore ordersapp.Store, entStore entitlementsapp.Store, evidenceProvider refundsapp.DeliveryEvidenceProvider, notifier refundsapp.Notifier) *PostgresStore {
	return &PostgresStore{
		pool:             pool,
		orderStore:       orderStore,
		entitlementStore: entStore,
		evidenceProvider: evidenceProvider,
		notifier:         notifier,
	}
}

func scanRefundRow(row pgx.Row) (*refundsdomain.RefundApplication, error) {
	var app refundsdomain.RefundApplication
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
	app.ApplicationState = refundsdomain.ApplicationState(appStateStr)
	app.ChannelState = refundsdomain.ChannelState(chStateStr)
	return &app, nil
}

const refundSelectCols = `
	id, order_id, user_id, reason, exception_type, is_out_of_window,
	amount_minor, application_state, channel_state, channel_refund_id, channel_failure_reason,
	reviewer_id, reviewed_at, rejection_reason, version, first_available_at,
	deadline, created_at, updated_at
`

func (s *PostgresStore) ApplyRefund(ctx context.Context, userID, orderID, reason, exceptionType string) (*refundsdomain.RefundApplication, error) {
	reason = strings.TrimSpace(reason)
	exceptionType = strings.TrimSpace(exceptionType)

	if exceptionType != "" && !refundsdomain.IsValidExceptionType(exceptionType) {
		return nil, refundsdomain.ErrInvalidExceptionType
	}

	order, err := s.orderStore.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, refundsdomain.ErrPermissionDenied
	}
	if order.Status != ordersdomain.StatusPaid {
		return nil, refundsdomain.ErrOrderNotEligible
	}

	now := time.Now().UTC()
	firstAvailableAt, deadline, isOutOfWindow, err := refundsapp.ComputeRefundWindow(ctx, order, s.evidenceProvider, now)
	if err != nil {
		return nil, err
	}
	if isOutOfWindow && exceptionType == "" {
		return nil, refundsdomain.ErrRefundWindowExpired
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
		return nil, ordersdomain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if ordersdomain.OrderStatus(ordStatus) != ordersdomain.StatusPaid {
		return nil, refundsdomain.ErrOrderNotEligible
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

	refID := refundsapp.NewID("ref")
	app, err := scanRefundRow(tx.QueryRow(ctx, `
		INSERT INTO refund_applications (
			id, order_id, user_id, reason, exception_type, is_out_of_window,
			amount_minor, application_state, channel_state, version,
			first_available_at, deadline, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1, $10, $11, $12, $12)
		RETURNING `+refundSelectCols,
		refID, orderID, userID, reason, exceptionType, isOutOfWindow,
		order.AmountMinor, string(refundsdomain.StatePendingReview), string(refundsdomain.ChannelNone),
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

func (s *PostgresStore) CancelRefund(ctx context.Context, userID, applicationID string, expectedVersion int) (*refundsdomain.RefundApplication, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	app, err := scanRefundRow(tx.QueryRow(ctx, `SELECT `+refundSelectCols+` FROM refund_applications WHERE id = $1 FOR UPDATE`, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, refundsdomain.ErrRefundNotFound
	}
	if err != nil {
		return nil, err
	}
	if app.UserID != userID {
		return nil, refundsdomain.ErrPermissionDenied
	}
	if app.ApplicationState != refundsdomain.StatePendingReview {
		return nil, refundsdomain.ErrRefundNotPending
	}
	if expectedVersion > 0 && app.Version != expectedVersion {
		return nil, refundsdomain.ErrVersionConflict
	}

	updated, err := scanRefundRow(tx.QueryRow(ctx, `
		UPDATE refund_applications
		SET application_state = $2, version = version + 1, updated_at = $3
		WHERE id = $1
		RETURNING `+refundSelectCols,
		applicationID, string(refundsdomain.StateCancelled), time.Now().UTC(),
	))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *PostgresStore) ReviewRefund(ctx context.Context, adminID, applicationID, decision, rejectionReason string, expectedVersion int) (*refundsdomain.RefundApplication, error) {
	if decision != "approved" && decision != "rejected" {
		return nil, refundsdomain.ErrInvalidReviewDecision
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	app, err := scanRefundRow(tx.QueryRow(ctx, `SELECT `+refundSelectCols+` FROM refund_applications WHERE id = $1 FOR UPDATE`, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, refundsdomain.ErrRefundNotFound
	}
	if err != nil {
		return nil, err
	}
	if app.ApplicationState != refundsdomain.StatePendingReview {
		return nil, refundsdomain.ErrRefundNotPending
	}
	if expectedVersion > 0 && app.Version != expectedVersion {
		return nil, refundsdomain.ErrVersionConflict
	}

	now := time.Now().UTC()
	nextAppState := refundsdomain.StateRejected
	nextChState := refundsdomain.ChannelNone
	rejReason := strings.TrimSpace(rejectionReason)
	if decision == "approved" {
		nextAppState = refundsdomain.StateApproved
		nextChState = refundsdomain.ChannelSubmitting
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

func (s *PostgresStore) ConfirmChannelRefund(ctx context.Context, applicationID string, status refundsdomain.ChannelState, channelRefundID, failureReason string, expectedVersion int) (*refundsdomain.RefundApplication, error) {
	if status != refundsdomain.ChannelSuccess && status != refundsdomain.ChannelFailed && status != refundsdomain.ChannelUnknown {
		return nil, refundsdomain.ErrInvalidChannelState
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	app, err := scanRefundRow(tx.QueryRow(ctx, `SELECT `+refundSelectCols+` FROM refund_applications WHERE id = $1 FOR UPDATE`, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, refundsdomain.ErrRefundNotFound
	}
	if err != nil {
		return nil, err
	}
	if app.ApplicationState != refundsdomain.StateApproved {
		return nil, refundsdomain.ErrRefundNotApproved
	}
	if expectedVersion > 0 && app.Version != expectedVersion {
		return nil, refundsdomain.ErrVersionConflict
	}

	if app.ChannelState == refundsdomain.ChannelSuccess {
		if status == refundsdomain.ChannelSuccess {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return app, nil
		}
		return nil, refundsdomain.ErrRefundTerminalState
	}

	now := time.Now().UTC()
	chRefID := app.ChannelRefundID
	if channelRefundID != "" {
		chRefID = channelRefundID
	}
	failReason := strings.TrimSpace(failureReason)
	if status == refundsdomain.ChannelSuccess {
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

	if status == refundsdomain.ChannelSuccess {
		refundsapp.SendRefundSucceededNotification(ctx, s.notifier, updated)
	}
	return updated, nil
}

func (s *PostgresStore) GetRefund(ctx context.Context, applicationID string) (*refundsdomain.RefundApplication, error) {
	app, err := scanRefundRow(s.pool.QueryRow(ctx, `SELECT `+refundSelectCols+` FROM refund_applications WHERE id = $1`, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, refundsdomain.ErrRefundNotFound
	}
	if err != nil {
		return nil, err
	}
	return app, nil
}

func (s *PostgresStore) ListRefunds(ctx context.Context, page, limit int) ([]*refundsdomain.RefundApplication, int, error) {
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

	list := make([]*refundsdomain.RefundApplication, 0)
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

func (s *PostgresStore) ListRefundsByOrder(ctx context.Context, orderID string) ([]*refundsdomain.RefundApplication, error) {
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

	list := make([]*refundsdomain.RefundApplication, 0)
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
