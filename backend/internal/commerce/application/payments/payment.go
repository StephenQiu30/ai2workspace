package payments

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	ordersapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/orders"
	ordersdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/orders"
	paymentsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/payments"
)

// EntitlementGranter activates a purchase grant tied to an order.
type EntitlementGranter func(ctx context.Context, userID, courseID, orderID string) error

// Notifier delivers deduplicated notifications to community. Failures never roll back payment facts.
type Notifier func(ctx context.Context, eventID, recipientID, notifType, title, summary string) error

// ChannelStatusQuerier actively queries the upstream payment channel for an attempt's true status.
type ChannelStatusQuerier func(ctx context.Context, attempt *paymentsdomain.PaymentAttempt) (*paymentsdomain.ChannelQueryResult, error)

// Store defines persistence operations for payment attempts and status reconciliation.
type Store interface {
	CreatePaymentAttempt(ctx context.Context, orderID, channel string, amountMinor int64, currency string) (*paymentsdomain.PaymentAttempt, error)
	GetPaymentAttempt(ctx context.Context, attemptID string) (*paymentsdomain.PaymentAttempt, error)
	ConfirmPayment(ctx context.Context, orderID, channel, channelOrderID string, amountMinor int64, currency, signature string) (*paymentsdomain.PaymentAttempt, error)
	ListAttemptsByOrder(ctx context.Context, orderID string) ([]*paymentsdomain.PaymentAttempt, error)
	MarkAttemptUnknown(ctx context.Context, attemptID string) (*paymentsdomain.PaymentAttempt, error)
	QueryPaymentStatus(ctx context.Context, orderID string) (*paymentsdomain.PaymentStatusReport, error)
	ListAnomalies(ctx context.Context, orderID string) ([]*paymentsdomain.PaymentAnomaly, error)
}

// ComputeWebhookSignature calculates the canonical HMAC-SHA256 hex digest for a payment webhook callback.
func ComputeWebhookSignature(secret, orderID, channel, channelOrderID string, amountMinor int64, currency string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	payload := fmt.Sprintf("%s|%s|%s|%d|%s", orderID, channel, channelOrderID, amountMinor, currency)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature validates a payment webhook signature in constant time.
func VerifyWebhookSignature(secret, orderID, channel, channelOrderID string, amountMinor int64, currency, provided string) bool {
	if secret == "" || strings.TrimSpace(provided) == "" {
		return false
	}
	expected := ComputeWebhookSignature(secret, orderID, channel, channelOrderID, amountMinor, currency)
	return hmac.Equal([]byte(strings.ToLower(strings.TrimSpace(provided))), []byte(expected))
}

func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func CloneAttempt(a *paymentsdomain.PaymentAttempt) *paymentsdomain.PaymentAttempt {
	if a == nil {
		return nil
	}
	cp := *a
	return &cp
}

func cloneAnomaly(a *paymentsdomain.PaymentAnomaly) *paymentsdomain.PaymentAnomaly {
	if a == nil {
		return nil
	}
	cp := *a
	return &cp
}

func SendPaymentNotification(ctx context.Context, notifier Notifier, order *ordersdomain.Order) {
	if notifier == nil || order == nil {
		return
	}
	eventID := fmt.Sprintf("commerce:order_paid:%s", order.ID)
	title := "课程购买成功"
	summary := fmt.Sprintf("订单 %s 已确认到账并开通学习权益。", order.ID)
	if order.CourseTitle != "" {
		summary = fmt.Sprintf("课程《%s》（订单 %s）已确认到账并开通学习权益。", order.CourseTitle, order.ID)
	}
	notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := notifier(notifyCtx, eventID, order.UserID, "order_paid", title, summary); err != nil {
		slog.Warn("failed to deliver payment notification", "orderId", order.ID, "eventId", eventID, "error", err)
	}
}

// StoreOptions groups optional collaborators for payment stores.
type StoreOptions struct {
	WebhookSecret string
	Notifier      Notifier
	Querier       ChannelStatusQuerier
}

// MemoryStore provides an in-memory thread-safe store for payment attempts.
type MemoryStore struct {
	mu            sync.RWMutex
	attempts      map[string]*paymentsdomain.PaymentAttempt // attemptID -> PaymentAttempt
	anomalies     []*paymentsdomain.PaymentAnomaly
	orderStore    ordersapp.Store
	granter       EntitlementGranter
	webhookSecret string
	notifier      Notifier
	querier       ChannelStatusQuerier
}

// NewMemoryStore initializes a new MemoryStore for payments.
func NewMemoryStore(orderStore ordersapp.Store, granter EntitlementGranter, opts StoreOptions) *MemoryStore {
	return &MemoryStore{
		attempts:      make(map[string]*paymentsdomain.PaymentAttempt),
		orderStore:    orderStore,
		granter:       granter,
		webhookSecret: opts.WebhookSecret,
		notifier:      opts.Notifier,
		querier:       opts.Querier,
	}
}

func (s *MemoryStore) CreatePaymentAttempt(_ context.Context, orderID, channel string, amountMinor int64, currency string) (*paymentsdomain.PaymentAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reuse existing unresolved attempt to prevent duplicate charges.
	for _, a := range s.attempts {
		if a.OrderID == orderID && (a.Status == paymentsdomain.StatusPending || a.Status == paymentsdomain.StatusUnknown) {
			return CloneAttempt(a), nil
		}
	}

	now := time.Now().UTC()
	attemptID := NewID("pay")
	chOrderID := NewID("ch_tx")

	attempt := &paymentsdomain.PaymentAttempt{
		ID:             attemptID,
		OrderID:        orderID,
		Channel:        channel,
		ChannelOrderID: chOrderID,
		AmountMinor:    amountMinor,
		Currency:       currency,
		Status:         paymentsdomain.StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.attempts[attemptID] = attempt
	return CloneAttempt(attempt), nil
}

func (s *MemoryStore) GetPaymentAttempt(_ context.Context, attemptID string) (*paymentsdomain.PaymentAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	att, ok := s.attempts[attemptID]
	if !ok {
		return nil, paymentsdomain.ErrPaymentNotFound
	}
	return CloneAttempt(att), nil
}

func (s *MemoryStore) MarkAttemptUnknown(_ context.Context, attemptID string) (*paymentsdomain.PaymentAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	att, ok := s.attempts[attemptID]
	if !ok {
		return nil, paymentsdomain.ErrPaymentNotFound
	}
	if att.Status == paymentsdomain.StatusSuccess || att.Status == paymentsdomain.StatusFailed {
		return nil, paymentsdomain.ErrOrderStateConflict
	}
	att.Status = paymentsdomain.StatusUnknown
	att.UpdatedAt = time.Now().UTC()
	return CloneAttempt(att), nil
}

func (s *MemoryStore) ConfirmPayment(ctx context.Context, orderID, channel, channelOrderID string, amountMinor int64, currency, signature string) (*paymentsdomain.PaymentAttempt, error) {
	if !VerifyWebhookSignature(s.webhookSecret, orderID, channel, channelOrderID, amountMinor, currency, signature) {
		return nil, paymentsdomain.ErrInvalidPaymentFacts
	}

	att, paidOrder, shouldNotify, err := s.confirmPaymentLocked(ctx, orderID, channel, channelOrderID, amountMinor, currency)
	if err != nil {
		return nil, err
	}
	if shouldNotify {
		SendPaymentNotification(ctx, s.notifier, paidOrder)
	}
	return att, nil
}

func (s *MemoryStore) confirmPaymentLocked(ctx context.Context, orderID, channel, channelOrderID string, amountMinor int64, currency string) (*paymentsdomain.PaymentAttempt, *ordersdomain.Order, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, err := s.orderStore.GetOrder(ctx, orderID)
	if err != nil {
		return nil, nil, false, err
	}

	if order.AmountMinor != amountMinor || order.Currency != currency {
		return nil, nil, false, paymentsdomain.ErrInvalidPaymentFacts
	}

	switch order.Status {
	case ordersdomain.StatusPaid:
		// Idempotent replay only for matching channel transaction ID.
		for _, a := range s.attempts {
			if a.OrderID == orderID && a.Status == paymentsdomain.StatusSuccess && a.ChannelOrderID == channelOrderID {
				return CloneAttempt(a), order, false, nil
			}
		}
		s.recordAnomalyLocked(orderID, channel, channelOrderID, amountMinor, currency, string(order.Status), "duplicate_payment_different_channel_order")
		return nil, nil, false, paymentsdomain.ErrOrderStateConflict

	case ordersdomain.StatusRefunded, ordersdomain.StatusCancelled:
		reason := fmt.Sprintf("late_payment_callback_on_%s_order", order.Status)
		s.recordAnomalyLocked(orderID, channel, channelOrderID, amountMinor, currency, string(order.Status), reason)
		return nil, nil, false, paymentsdomain.ErrOrderStateConflict

	case ordersdomain.StatusPendingPayment:
		// Allowed transition: pending_payment -> paid.
	default:
		s.recordAnomalyLocked(orderID, channel, channelOrderID, amountMinor, currency, string(order.Status), "unexpected_order_status")
		return nil, nil, false, paymentsdomain.ErrOrderStateConflict
	}

	now := time.Now().UTC()
	paidOrder, err := s.orderStore.MarkOrderPaid(ctx, orderID, now)
	if err != nil {
		if errors.Is(err, ordersdomain.ErrOrderNotPending) {
			s.recordAnomalyLocked(orderID, channel, channelOrderID, amountMinor, currency, "concurrent_transition", "order_no_longer_pending_payment")
			return nil, nil, false, paymentsdomain.ErrOrderStateConflict
		}
		return nil, nil, false, err
	}

	if s.granter != nil {
		if err := s.granter(ctx, paidOrder.UserID, paidOrder.CourseID, orderID); err != nil {
			return nil, nil, false, fmt.Errorf("grant entitlement failed: %w", err)
		}
	}

	var targetAttempt *paymentsdomain.PaymentAttempt
	for _, a := range s.attempts {
		if a.OrderID == orderID && (a.Status == paymentsdomain.StatusPending || a.Status == paymentsdomain.StatusUnknown) {
			targetAttempt = a
			break
		}
	}

	if targetAttempt == nil {
		targetAttempt = &paymentsdomain.PaymentAttempt{
			ID:          NewID("pay"),
			OrderID:     orderID,
			Channel:     channel,
			AmountMinor: amountMinor,
			Currency:    currency,
			CreatedAt:   now,
		}
		s.attempts[targetAttempt.ID] = targetAttempt
	}

	targetAttempt.Status = paymentsdomain.StatusSuccess
	targetAttempt.Channel = channel
	targetAttempt.ChannelOrderID = channelOrderID
	targetAttempt.UpdatedAt = now

	return CloneAttempt(targetAttempt), paidOrder, true, nil
}

func (s *MemoryStore) recordAnomalyLocked(orderID, channel, channelOrderID string, amountMinor int64, currency, orderStatus, reason string) {
	anomaly := &paymentsdomain.PaymentAnomaly{
		ID:             NewID("anom"),
		OrderID:        orderID,
		Channel:        channel,
		ChannelOrderID: channelOrderID,
		AmountMinor:    amountMinor,
		Currency:       currency,
		OrderStatus:    orderStatus,
		Reason:         reason,
		RecordedAt:     time.Now().UTC(),
	}
	s.anomalies = append(s.anomalies, anomaly)
	slog.Warn("payment callback anomaly recorded",
		"anomalyId", anomaly.ID,
		"orderId", orderID,
		"channelOrderId", channelOrderID,
		"orderStatus", orderStatus,
		"reason", reason,
	)
}

func (s *MemoryStore) ListAttemptsByOrder(_ context.Context, orderID string) ([]*paymentsdomain.PaymentAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*paymentsdomain.PaymentAttempt, 0)
	for _, a := range s.attempts {
		if a.OrderID == orderID {
			list = append(list, CloneAttempt(a))
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID > list[j].ID
		}
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *MemoryStore) QueryPaymentStatus(ctx context.Context, orderID string) (*paymentsdomain.PaymentStatusReport, error) {
	order, err := s.orderStore.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	attempts, err := s.ListAttemptsByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var latest *paymentsdomain.PaymentAttempt
	if len(attempts) > 0 {
		latest = attempts[0]
	}

	// If order is still pending_payment and latest attempt is unknown or pending, actively query the channel.
	if order.Status == ordersdomain.StatusPendingPayment && latest != nil &&
		(latest.Status == paymentsdomain.StatusUnknown || latest.Status == paymentsdomain.StatusPending) && s.querier != nil {
		res, qErr := s.querier(ctx, CloneAttempt(latest))
		if qErr != nil {
			// Mark attempt unknown when channel query fails; never fabricate success.
			s.mu.Lock()
			if att, ok := s.attempts[latest.ID]; ok && att.Status == paymentsdomain.StatusPending {
				att.Status = paymentsdomain.StatusUnknown
				att.UpdatedAt = now
				latest = CloneAttempt(att)
			}
			s.mu.Unlock()
			return &paymentsdomain.PaymentStatusReport{
				OrderID:       order.ID,
				OrderStatus:   string(order.Status),
				PaymentStatus: string(paymentsdomain.StatusUnknown),
				LatestAttempt: latest,
				Recovered:     false,
				Message:       "payment channel query failed; status remains unknown pending reconciliation",
				CheckedAt:     now,
			}, nil
		}

		if res != nil {
			switch res.Status {
			case paymentsdomain.StatusSuccess:
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
				confirmed, paidOrder, shouldNotify, cErr := s.confirmPaymentLocked(ctx, order.ID, latest.Channel, chOrderID, amount, cur)
				if cErr != nil {
					return nil, cErr
				}
				if shouldNotify {
					SendPaymentNotification(ctx, s.notifier, paidOrder)
				}
				return &paymentsdomain.PaymentStatusReport{
					OrderID:       paidOrder.ID,
					OrderStatus:   string(paidOrder.Status),
					PaymentStatus: string(confirmed.Status),
					LatestAttempt: confirmed,
					Recovered:     true,
					Message:       "payment confirmed via active channel inquiry",
					CheckedAt:     now,
				}, nil
			case paymentsdomain.StatusFailed:
				s.mu.Lock()
				if att, ok := s.attempts[latest.ID]; ok && (att.Status == paymentsdomain.StatusPending || att.Status == paymentsdomain.StatusUnknown) {
					att.Status = paymentsdomain.StatusFailed
					att.UpdatedAt = now
					latest = CloneAttempt(att)
				}
				s.mu.Unlock()
				return &paymentsdomain.PaymentStatusReport{
					OrderID:       order.ID,
					OrderStatus:   string(order.Status),
					PaymentStatus: string(paymentsdomain.StatusFailed),
					LatestAttempt: latest,
					Recovered:     true,
					Message:       "payment confirmed failed via active channel inquiry",
					CheckedAt:     now,
				}, nil
			case paymentsdomain.StatusUnknown:
				s.mu.Lock()
				if att, ok := s.attempts[latest.ID]; ok && att.Status == paymentsdomain.StatusPending {
					att.Status = paymentsdomain.StatusUnknown
					att.UpdatedAt = now
					latest = CloneAttempt(att)
				}
				s.mu.Unlock()
			}
		}
	}

	payStatus := "none"
	if latest != nil {
		payStatus = string(latest.Status)
	} else if order.Status == ordersdomain.StatusPaid || order.Status == ordersdomain.StatusRefunded {
		payStatus = string(paymentsdomain.StatusSuccess)
	}

	return &paymentsdomain.PaymentStatusReport{
		OrderID:       order.ID,
		OrderStatus:   string(order.Status),
		PaymentStatus: payStatus,
		LatestAttempt: latest,
		Recovered:     false,
		CheckedAt:     now,
	}, nil
}

func (s *MemoryStore) ListAnomalies(_ context.Context, orderID string) ([]*paymentsdomain.PaymentAnomaly, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*paymentsdomain.PaymentAnomaly, 0)
	for _, a := range s.anomalies {
		if orderID == "" || a.OrderID == orderID {
			list = append(list, cloneAnomaly(a))
		}
	}
	return list, nil
}
