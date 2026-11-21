// Package payments manages payment attempts, webhook verification, out-of-order protection, and status recovery.
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

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
)

var (
	ErrPaymentNotFound     = errors.New("payment attempt not found")
	ErrInvalidPaymentFacts = errors.New("payment facts mismatch: amount, currency, or signature mismatch")
	ErrOrderStateConflict  = errors.New("order state conflict: payment callback rejected")
	ErrOrderNotPending     = errors.New("order is not in pending_payment state")
)

// PaymentStatus indicates state of a payment attempt.
type PaymentStatus string

const (
	StatusPending PaymentStatus = "pending"
	StatusSuccess PaymentStatus = "success"
	StatusFailed  PaymentStatus = "failed"
	StatusUnknown PaymentStatus = "unknown"
)

// PaymentAttempt records a transaction attempt through a payment channel.
type PaymentAttempt struct {
	ID             string        `json:"id"`
	OrderID        string        `json:"orderId"`
	Channel        string        `json:"channel"`
	ChannelOrderID string        `json:"channelOrderId"`
	AmountMinor    int64         `json:"amountMinor"`
	Currency       string        `json:"currency"`
	Status         PaymentStatus `json:"status"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
}

// PaymentAnomaly records rejected out-of-order or duplicate payment callbacks for audit and reconciliation.
type PaymentAnomaly struct {
	ID             string    `json:"id"`
	OrderID        string    `json:"orderId"`
	Channel        string    `json:"channel"`
	ChannelOrderID string    `json:"channelOrderId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	OrderStatus    string    `json:"orderStatus"`
	Reason         string    `json:"reason"`
	RecordedAt     time.Time `json:"recordedAt"`
}

// ChannelQueryResult represents an active payment status inquiry result from the payment channel.
type ChannelQueryResult struct {
	Status         PaymentStatus
	ChannelOrderID string
	AmountMinor    int64
	Currency       string
}

// PaymentStatusReport summarizes the reconciled payment state for an order.
type PaymentStatusReport struct {
	OrderID       string          `json:"orderId"`
	OrderStatus   string          `json:"orderStatus"`
	PaymentStatus string          `json:"paymentStatus"`
	LatestAttempt *PaymentAttempt `json:"latestAttempt,omitempty"`
	Recovered     bool            `json:"recovered"`
	Message       string          `json:"message,omitempty"`
	CheckedAt     time.Time       `json:"checkedAt"`
}

// EntitlementGranter activates a purchase grant tied to an order.
type EntitlementGranter func(ctx context.Context, userID, courseID, orderID string) error

// Notifier delivers deduplicated notifications to community. Failures never roll back payment facts.
type Notifier func(ctx context.Context, eventID, recipientID, notifType, title, summary string) error

// ChannelStatusQuerier actively queries the upstream payment channel for an attempt's true status.
type ChannelStatusQuerier func(ctx context.Context, attempt *PaymentAttempt) (*ChannelQueryResult, error)

// Store defines persistence operations for payment attempts and status reconciliation.
type Store interface {
	CreatePaymentAttempt(ctx context.Context, orderID, channel string, amountMinor int64, currency string) (*PaymentAttempt, error)
	GetPaymentAttempt(ctx context.Context, attemptID string) (*PaymentAttempt, error)
	ConfirmPayment(ctx context.Context, orderID, channel, channelOrderID string, amountMinor int64, currency, signature string) (*PaymentAttempt, error)
	ListAttemptsByOrder(ctx context.Context, orderID string) ([]*PaymentAttempt, error)
	MarkAttemptUnknown(ctx context.Context, attemptID string) (*PaymentAttempt, error)
	QueryPaymentStatus(ctx context.Context, orderID string) (*PaymentStatusReport, error)
	ListAnomalies(ctx context.Context, orderID string) ([]*PaymentAnomaly, error)
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

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func cloneAttempt(a *PaymentAttempt) *PaymentAttempt {
	if a == nil {
		return nil
	}
	cp := *a
	return &cp
}

func cloneAnomaly(a *PaymentAnomaly) *PaymentAnomaly {
	if a == nil {
		return nil
	}
	cp := *a
	return &cp
}

func sendPaymentNotification(ctx context.Context, notifier Notifier, order *orders.Order) {
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
	attempts      map[string]*PaymentAttempt // attemptID -> PaymentAttempt
	anomalies     []*PaymentAnomaly
	orderStore    orders.Store
	granter       EntitlementGranter
	webhookSecret string
	notifier      Notifier
	querier       ChannelStatusQuerier
}

// NewMemoryStore initializes a new MemoryStore for payments.
func NewMemoryStore(orderStore orders.Store, granter EntitlementGranter, opts StoreOptions) *MemoryStore {
	secret := opts.WebhookSecret
	if secret == "" {
		secret = "dev-commerce-webhook-secret"
	}
	return &MemoryStore{
		attempts:      make(map[string]*PaymentAttempt),
		orderStore:    orderStore,
		granter:       granter,
		webhookSecret: secret,
		notifier:      opts.Notifier,
		querier:       opts.Querier,
	}
}

func (s *MemoryStore) CreatePaymentAttempt(_ context.Context, orderID, channel string, amountMinor int64, currency string) (*PaymentAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reuse existing unresolved attempt to prevent duplicate charges.
	for _, a := range s.attempts {
		if a.OrderID == orderID && (a.Status == StatusPending || a.Status == StatusUnknown) {
			return cloneAttempt(a), nil
		}
	}

	now := time.Now().UTC()
	attemptID := generateID("pay")
	chOrderID := generateID("ch_tx")

	attempt := &PaymentAttempt{
		ID:             attemptID,
		OrderID:        orderID,
		Channel:        channel,
		ChannelOrderID: chOrderID,
		AmountMinor:    amountMinor,
		Currency:       currency,
		Status:         StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.attempts[attemptID] = attempt
	return cloneAttempt(attempt), nil
}

func (s *MemoryStore) GetPaymentAttempt(_ context.Context, attemptID string) (*PaymentAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	att, ok := s.attempts[attemptID]
	if !ok {
		return nil, ErrPaymentNotFound
	}
	return cloneAttempt(att), nil
}

func (s *MemoryStore) MarkAttemptUnknown(_ context.Context, attemptID string) (*PaymentAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	att, ok := s.attempts[attemptID]
	if !ok {
		return nil, ErrPaymentNotFound
	}
	if att.Status == StatusSuccess || att.Status == StatusFailed {
		return nil, ErrOrderStateConflict
	}
	att.Status = StatusUnknown
	att.UpdatedAt = time.Now().UTC()
	return cloneAttempt(att), nil
}

func (s *MemoryStore) ConfirmPayment(ctx context.Context, orderID, channel, channelOrderID string, amountMinor int64, currency, signature string) (*PaymentAttempt, error) {
	if !VerifyWebhookSignature(s.webhookSecret, orderID, channel, channelOrderID, amountMinor, currency, signature) {
		return nil, ErrInvalidPaymentFacts
	}

	att, paidOrder, shouldNotify, err := s.confirmPaymentLocked(ctx, orderID, channel, channelOrderID, amountMinor, currency)
	if err != nil {
		return nil, err
	}
	if shouldNotify {
		sendPaymentNotification(ctx, s.notifier, paidOrder)
	}
	return att, nil
}

func (s *MemoryStore) confirmPaymentLocked(ctx context.Context, orderID, channel, channelOrderID string, amountMinor int64, currency string) (*PaymentAttempt, *orders.Order, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, err := s.orderStore.GetOrder(ctx, orderID)
	if err != nil {
		return nil, nil, false, err
	}

	if order.AmountMinor != amountMinor || order.Currency != currency {
		return nil, nil, false, ErrInvalidPaymentFacts
	}

	switch order.Status {
	case orders.StatusPaid:
		// Idempotent replay only for matching channel transaction ID.
		for _, a := range s.attempts {
			if a.OrderID == orderID && a.Status == StatusSuccess && a.ChannelOrderID == channelOrderID {
				return cloneAttempt(a), order, false, nil
			}
		}
		s.recordAnomalyLocked(orderID, channel, channelOrderID, amountMinor, currency, string(order.Status), "duplicate_payment_different_channel_order")
		return nil, nil, false, ErrOrderStateConflict

	case orders.StatusRefunded, orders.StatusCancelled:
		reason := fmt.Sprintf("late_payment_callback_on_%s_order", order.Status)
		s.recordAnomalyLocked(orderID, channel, channelOrderID, amountMinor, currency, string(order.Status), reason)
		return nil, nil, false, ErrOrderStateConflict

	case orders.StatusPendingPayment:
		// Allowed transition: pending_payment -> paid.
	default:
		s.recordAnomalyLocked(orderID, channel, channelOrderID, amountMinor, currency, string(order.Status), "unexpected_order_status")
		return nil, nil, false, ErrOrderStateConflict
	}

	now := time.Now().UTC()
	paidOrder, err := s.orderStore.MarkOrderPaid(ctx, orderID, now)
	if err != nil {
		if errors.Is(err, orders.ErrOrderNotPending) {
			s.recordAnomalyLocked(orderID, channel, channelOrderID, amountMinor, currency, "concurrent_transition", "order_no_longer_pending_payment")
			return nil, nil, false, ErrOrderStateConflict
		}
		return nil, nil, false, err
	}

	if s.granter != nil {
		if err := s.granter(ctx, paidOrder.UserID, paidOrder.CourseID, orderID); err != nil {
			return nil, nil, false, fmt.Errorf("grant entitlement failed: %w", err)
		}
	}

	var targetAttempt *PaymentAttempt
	for _, a := range s.attempts {
		if a.OrderID == orderID && (a.Status == StatusPending || a.Status == StatusUnknown) {
			targetAttempt = a
			break
		}
	}

	if targetAttempt == nil {
		targetAttempt = &PaymentAttempt{
			ID:          generateID("pay"),
			OrderID:     orderID,
			Channel:     channel,
			AmountMinor: amountMinor,
			Currency:    currency,
			CreatedAt:   now,
		}
		s.attempts[targetAttempt.ID] = targetAttempt
	}

	targetAttempt.Status = StatusSuccess
	targetAttempt.Channel = channel
	targetAttempt.ChannelOrderID = channelOrderID
	targetAttempt.UpdatedAt = now

	return cloneAttempt(targetAttempt), paidOrder, true, nil
}

func (s *MemoryStore) recordAnomalyLocked(orderID, channel, channelOrderID string, amountMinor int64, currency, orderStatus, reason string) {
	anomaly := &PaymentAnomaly{
		ID:             generateID("anom"),
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

func (s *MemoryStore) ListAttemptsByOrder(_ context.Context, orderID string) ([]*PaymentAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*PaymentAttempt, 0)
	for _, a := range s.attempts {
		if a.OrderID == orderID {
			list = append(list, cloneAttempt(a))
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

func (s *MemoryStore) QueryPaymentStatus(ctx context.Context, orderID string) (*PaymentStatusReport, error) {
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

	// If order is still pending_payment and latest attempt is unknown or pending, actively query the channel.
	if order.Status == orders.StatusPendingPayment && latest != nil &&
		(latest.Status == StatusUnknown || latest.Status == StatusPending) && s.querier != nil {
		res, qErr := s.querier(ctx, cloneAttempt(latest))
		if qErr != nil {
			// Mark attempt unknown when channel query fails; never fabricate success.
			s.mu.Lock()
			if att, ok := s.attempts[latest.ID]; ok && att.Status == StatusPending {
				att.Status = StatusUnknown
				att.UpdatedAt = now
				latest = cloneAttempt(att)
			}
			s.mu.Unlock()
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
				confirmed, paidOrder, shouldNotify, cErr := s.confirmPaymentLocked(ctx, order.ID, latest.Channel, chOrderID, amount, cur)
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
				s.mu.Lock()
				if att, ok := s.attempts[latest.ID]; ok && (att.Status == StatusPending || att.Status == StatusUnknown) {
					att.Status = StatusFailed
					att.UpdatedAt = now
					latest = cloneAttempt(att)
				}
				s.mu.Unlock()
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
				s.mu.Lock()
				if att, ok := s.attempts[latest.ID]; ok && att.Status == StatusPending {
					att.Status = StatusUnknown
					att.UpdatedAt = now
					latest = cloneAttempt(att)
				}
				s.mu.Unlock()
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

func (s *MemoryStore) ListAnomalies(_ context.Context, orderID string) ([]*PaymentAnomaly, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*PaymentAnomaly, 0)
	for _, a := range s.anomalies {
		if orderID == "" || a.OrderID == orderID {
			list = append(list, cloneAnomaly(a))
		}
	}
	return list, nil
}
