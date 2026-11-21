package payments

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store backed by PostgreSQL via pgxpool.
type PostgresStore struct {
	pool          *pgxpool.Pool
	orderStore    orders.Store
	granter       EntitlementGranter
	webhookSecret string
	notifier      Notifier
	querier       ChannelStatusQuerier
}

// NewPostgresStore creates a PostgreSQL-backed payments store.
func NewPostgresStore(pool *pgxpool.Pool, orderStore orders.Store, granter EntitlementGranter, opts StoreOptions) *PostgresStore {
	secret := opts.WebhookSecret
	if secret == "" {
		secret = "dev-commerce-webhook-secret"
	}
	return &PostgresStore{
		pool:          pool,
		orderStore:    orderStore,
		granter:       granter,
		webhookSecret: secret,
		notifier:      opts.Notifier,
		querier:       opts.Querier,
	}
}

func (s *PostgresStore) CreatePaymentAttempt(ctx context.Context, orderID, channel string, amountMinor int64, currency string) (*PaymentAttempt, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Reuse unresolved attempt if one exists.
	var existing PaymentAttempt
	var statusStr string
	err = tx.QueryRow(ctx, `
		SELECT id, order_id, channel, channel_order_id, amount_minor, currency, status, created_at, updated_at
		FROM payment_attempts
		WHERE order_id = $1 AND status IN ('pending', 'unknown')
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, orderID).Scan(
		&existing.ID, &existing.OrderID, &existing.Channel, &existing.ChannelOrderID,
		&existing.AmountMinor, &existing.Currency, &statusStr, &existing.CreatedAt, &existing.UpdatedAt,
	)
	if err == nil {
		existing.Status = PaymentStatus(statusStr)
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	now := time.Now().UTC()
	attempt := PaymentAttempt{
		ID:             generateID("pay"),
		OrderID:        orderID,
		Channel:        channel,
		ChannelOrderID: generateID("ch_tx"),
		AmountMinor:    amountMinor,
		Currency:       currency,
		Status:         StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO payment_attempts (id, order_id, channel, channel_order_id, amount_minor, currency, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, attempt.ID, attempt.OrderID, attempt.Channel, attempt.ChannelOrderID,
		attempt.AmountMinor, attempt.Currency, string(attempt.Status), attempt.CreatedAt, attempt.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (s *PostgresStore) GetPaymentAttempt(ctx context.Context, attemptID string) (*PaymentAttempt, error) {
	var att PaymentAttempt
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, order_id, channel, channel_order_id, amount_minor, currency, status, created_at, updated_at
		FROM payment_attempts WHERE id = $1
	`, attemptID).Scan(
		&att.ID, &att.OrderID, &att.Channel, &att.ChannelOrderID,
		&att.AmountMinor, &att.Currency, &statusStr, &att.CreatedAt, &att.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	att.Status = PaymentStatus(statusStr)
	return &att, nil
}

func (s *PostgresStore) MarkAttemptUnknown(ctx context.Context, attemptID string) (*PaymentAttempt, error) {
	var att PaymentAttempt
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		UPDATE payment_attempts
		SET status = $2, updated_at = $3
		WHERE id = $1 AND status IN ('pending', 'unknown')
		RETURNING id, order_id, channel, channel_order_id, amount_minor, currency, status, created_at, updated_at
	`, attemptID, string(StatusUnknown), time.Now().UTC()).Scan(
		&att.ID, &att.OrderID, &att.Channel, &att.ChannelOrderID,
		&att.AmountMinor, &att.Currency, &statusStr, &att.CreatedAt, &att.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, lookupErr := s.GetPaymentAttempt(ctx, attemptID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing.Status == StatusSuccess || existing.Status == StatusFailed {
			return nil, ErrOrderStateConflict
		}
		return nil, ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	att.Status = PaymentStatus(statusStr)
	return &att, nil
}

func (s *PostgresStore) ConfirmPayment(ctx context.Context, orderID, channel, channelOrderID string, amountMinor int64, currency, signature string) (*PaymentAttempt, error) {
	if !VerifyWebhookSignature(s.webhookSecret, orderID, channel, channelOrderID, amountMinor, currency, signature) {
		return nil, ErrInvalidPaymentFacts
	}

	att, paidOrder, shouldNotify, err := s.confirmPaymentTx(ctx, orderID, channel, channelOrderID, amountMinor, currency)
	if err != nil {
		return nil, err
	}
	if shouldNotify {
		sendPaymentNotification(ctx, s.notifier, paidOrder)
	}
	return att, nil
}

func (s *PostgresStore) confirmPaymentTx(ctx context.Context, orderID, channel, channelOrderID string, amountMinor int64, currency string) (*PaymentAttempt, *orders.Order, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ord orders.Order
	var statusStr string
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, course_id, course_title, amount_minor, currency, status, idempotency_key, policy_version, paid_at, created_at, updated_at
		FROM orders WHERE id = $1 FOR UPDATE
	`, orderID).Scan(
		&ord.ID, &ord.UserID, &ord.CourseID, &ord.CourseTitle, &ord.AmountMinor, &ord.Currency,
		&statusStr, &ord.IdempotencyKey, &ord.PolicyVersion, &ord.PaidAt, &ord.CreatedAt, &ord.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, false, orders.ErrOrderNotFound
	}
	if err != nil {
		return nil, nil, false, err
	}
	ord.Status = orders.OrderStatus(statusStr)

	if ord.AmountMinor != amountMinor || ord.Currency != currency {
		return nil, nil, false, ErrInvalidPaymentFacts
	}

	switch ord.Status {
	case orders.StatusPaid:
		var att PaymentAttempt
		var attStatus string
		err = tx.QueryRow(ctx, `
			SELECT id, order_id, channel, channel_order_id, amount_minor, currency, status, created_at, updated_at
			FROM payment_attempts
			WHERE order_id = $1 AND status = 'success' AND channel_order_id = $2
			LIMIT 1
		`, orderID, channelOrderID).Scan(
			&att.ID, &att.OrderID, &att.Channel, &att.ChannelOrderID,
			&att.AmountMinor, &att.Currency, &attStatus, &att.CreatedAt, &att.UpdatedAt,
		)
		if err == nil {
			att.Status = PaymentStatus(attStatus)
			if err := tx.Commit(ctx); err != nil {
				return nil, nil, false, err
			}
			return &att, &ord, false, nil
		}
		if err := s.recordAnomalyTx(ctx, tx, orderID, channel, channelOrderID, amountMinor, currency, string(ord.Status), "duplicate_payment_different_channel_order"); err != nil {
			return nil, nil, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, nil, false, err
		}
		return nil, nil, false, ErrOrderStateConflict

	case orders.StatusRefunded, orders.StatusCancelled:
		reason := fmt.Sprintf("late_payment_callback_on_%s_order", ord.Status)
		if err := s.recordAnomalyTx(ctx, tx, orderID, channel, channelOrderID, amountMinor, currency, string(ord.Status), reason); err != nil {
			return nil, nil, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, nil, false, err
		}
		return nil, nil, false, ErrOrderStateConflict

	case orders.StatusPendingPayment:
		// Valid state transition pending_payment -> paid.
	default:
		if err := s.recordAnomalyTx(ctx, tx, orderID, channel, channelOrderID, amountMinor, currency, string(ord.Status), "unexpected_order_status"); err != nil {
			return nil, nil, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, nil, false, err
		}
		return nil, nil, false, ErrOrderStateConflict
	}

	now := time.Now().UTC()
	var att PaymentAttempt
	var attStatus string
	err = tx.QueryRow(ctx, `
		SELECT id, order_id, channel, channel_order_id, amount_minor, currency, status, created_at, updated_at
		FROM payment_attempts
		WHERE order_id = $1 AND status IN ('pending', 'unknown')
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, orderID).Scan(
		&att.ID, &att.OrderID, &att.Channel, &att.ChannelOrderID,
		&att.AmountMinor, &att.Currency, &attStatus, &att.CreatedAt, &att.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		att = PaymentAttempt{
			ID:             generateID("pay"),
			OrderID:        orderID,
			Channel:        channel,
			ChannelOrderID: channelOrderID,
			AmountMinor:    amountMinor,
			Currency:       currency,
			Status:         StatusSuccess,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO payment_attempts (id, order_id, channel, channel_order_id, amount_minor, currency, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, att.ID, att.OrderID, att.Channel, att.ChannelOrderID, att.AmountMinor, att.Currency, string(StatusSuccess), now, now)
		if err != nil {
			return nil, nil, false, err
		}
	} else if err != nil {
		return nil, nil, false, err
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE payment_attempts
			SET status = $2, channel = $3, channel_order_id = $4, updated_at = $5
			WHERE id = $1
		`, att.ID, string(StatusSuccess), channel, channelOrderID, now)
		if err != nil {
			return nil, nil, false, err
		}
		att.Status = StatusSuccess
		att.Channel = channel
		att.ChannelOrderID = channelOrderID
		att.UpdatedAt = now
	}

	tag, err := tx.Exec(ctx, `
		UPDATE orders SET status = $2, paid_at = $3, updated_at = $4
		WHERE id = $1 AND status = $5
	`, orderID, string(orders.StatusPaid), now, now, string(orders.StatusPendingPayment))
	if err != nil {
		return nil, nil, false, err
	}
	if tag.RowsAffected() == 0 {
		return nil, nil, false, ErrOrderStateConflict
	}

	entID := generateID("ent")
	_, err = tx.Exec(ctx, `
		INSERT INTO entitlements (id, user_id, course_id, order_id, status, grant_version, activated_at)
		VALUES ($1, $2, $3, $4, 'active', 1, $5)
		ON CONFLICT (order_id) DO NOTHING
	`, entID, ord.UserID, ord.CourseID, orderID, now)
	if err != nil {
		return nil, nil, false, fmt.Errorf("grant entitlement failed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, false, err
	}

	ord.Status = orders.StatusPaid
	ord.PaidAt = &now
	ord.UpdatedAt = now
	return &att, &ord, true, nil
}

func (s *PostgresStore) recordAnomalyTx(ctx context.Context, tx pgx.Tx, orderID, channel, channelOrderID string, amountMinor int64, currency, orderStatus, reason string) error {
	id := generateID("anom")
	now := time.Now().UTC()
	_, err := tx.Exec(ctx, `
		INSERT INTO payment_anomalies (id, order_id, channel, channel_order_id, amount_minor, currency, order_status, reason, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, id, orderID, channel, channelOrderID, amountMinor, currency, orderStatus, reason, now)
	if err != nil {
		return err
	}
	slog.Warn("payment callback anomaly recorded",
		"anomalyId", id,
		"orderId", orderID,
		"channelOrderId", channelOrderID,
		"orderStatus", orderStatus,
		"reason", reason,
	)
	return nil
}

func (s *PostgresStore) ListAttemptsByOrder(ctx context.Context, orderID string) ([]*PaymentAttempt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, order_id, channel, channel_order_id, amount_minor, currency, status, created_at, updated_at
		FROM payment_attempts
		WHERE order_id = $1
		ORDER BY created_at DESC, id DESC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*PaymentAttempt, 0)
	for rows.Next() {
		var att PaymentAttempt
		var statusStr string
		if err := rows.Scan(
			&att.ID, &att.OrderID, &att.Channel, &att.ChannelOrderID,
			&att.AmountMinor, &att.Currency, &statusStr, &att.CreatedAt, &att.UpdatedAt,
		); err != nil {
			return nil, err
		}
		att.Status = PaymentStatus(statusStr)
		list = append(list, &att)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *PostgresStore) QueryPaymentStatus(ctx context.Context, orderID string) (*PaymentStatusReport, error) {
	order, err := s.orderStore.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	attempts, err := s.ListAttemptsByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var latest *PaymentAttempt
	if len(attempts) > 0 {
		latest = attempts[0]
	}

	if order.Status == orders.StatusPendingPayment && latest != nil &&
		(latest.Status == StatusUnknown || latest.Status == StatusPending) && s.querier != nil {
		res, qErr := s.querier(ctx, cloneAttempt(latest))
		if qErr != nil {
			if latest.Status == StatusPending {
				if updated, uErr := s.MarkAttemptUnknown(ctx, latest.ID); uErr == nil {
					latest = updated
				}
			}
			return &PaymentStatusReport{
				OrderID:       order.ID,
				OrderStatus:   string(order.Status),
				PaymentStatus: string(StatusUnknown),
				LatestAttempt: latest,
				Recovered:     false,
				Message:       "payment channel query failed; status remains unknown pending reconciliation",
				CheckedAt:     now,
			}, nil
		}

		if res != nil {
			switch res.Status {
			case StatusSuccess:
				chOrderID := res.ChannelOrderID
				if chOrderID == "" {
					chOrderID = latest.ChannelOrderID
				}
				amount := res.AmountMinor
				if amount == 0 {
					amount = latest.AmountMinor
				}
				cur := res.Currency
				if cur == "" {
					cur = latest.Currency
				}
				confirmed, paidOrder, shouldNotify, cErr := s.confirmPaymentTx(ctx, order.ID, latest.Channel, chOrderID, amount, cur)
				if cErr != nil {
					return nil, cErr
				}
				if shouldNotify {
					sendPaymentNotification(ctx, s.notifier, paidOrder)
				}
				return &PaymentStatusReport{
					OrderID:       paidOrder.ID,
					OrderStatus:   string(paidOrder.Status),
					PaymentStatus: string(confirmed.Status),
					LatestAttempt: confirmed,
					Recovered:     true,
					Message:       "payment confirmed via active channel inquiry",
					CheckedAt:     now,
				}, nil
			case StatusFailed:
				_, uErr := s.pool.Exec(ctx, `
					UPDATE payment_attempts SET status = $2, updated_at = $3
					WHERE id = $1 AND status IN ('pending', 'unknown')
				`, latest.ID, string(StatusFailed), now)
				if uErr != nil {
					return nil, uErr
				}
				latest.Status = StatusFailed
				latest.UpdatedAt = now
				return &PaymentStatusReport{
					OrderID:       order.ID,
					OrderStatus:   string(order.Status),
					PaymentStatus: string(StatusFailed),
					LatestAttempt: latest,
					Recovered:     true,
					Message:       "payment confirmed failed via active channel inquiry",
					CheckedAt:     now,
				}, nil
			case StatusUnknown:
				if latest.Status == StatusPending {
					if updated, uErr := s.MarkAttemptUnknown(ctx, latest.ID); uErr == nil {
						latest = updated
					}
				}
			}
		}
	}

	payStatus := "none"
	if latest != nil {
		payStatus = string(latest.Status)
	} else if order.Status == orders.StatusPaid || order.Status == orders.StatusRefunded {
		payStatus = string(StatusSuccess)
	}

	return &PaymentStatusReport{
		OrderID:       order.ID,
		OrderStatus:   string(order.Status),
		PaymentStatus: payStatus,
		LatestAttempt: latest,
		Recovered:     false,
		CheckedAt:     now,
	}, nil
}

func (s *PostgresStore) ListAnomalies(ctx context.Context, orderID string) ([]*PaymentAnomaly, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if orderID == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, order_id, channel, channel_order_id, amount_minor, currency, order_status, reason, recorded_at
			FROM payment_anomalies ORDER BY recorded_at DESC
		`)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, order_id, channel, channel_order_id, amount_minor, currency, order_status, reason, recorded_at
			FROM payment_anomalies WHERE order_id = $1 ORDER BY recorded_at DESC
		`, orderID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*PaymentAnomaly, 0)
	for rows.Next() {
		var a PaymentAnomaly
		if err := rows.Scan(
			&a.ID, &a.OrderID, &a.Channel, &a.ChannelOrderID,
			&a.AmountMinor, &a.Currency, &a.OrderStatus, &a.Reason, &a.RecordedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
