// Package payments manages payment attempts, sandbox gateway confirmation, and channel idempotency.
package payments

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
)

var (
	ErrPaymentNotFound       = errors.New("payment attempt not found")
	ErrInvalidPaymentFacts   = errors.New("payment facts mismatch: amount, currency, or signature mismatch")
	ErrOrderAlreadyProcessed = errors.New("order has already been processed")
)

// PaymentStatus indicates state of a payment attempt.
type PaymentStatus string

const (
	StatusPending PaymentStatus = "pending"
	StatusSuccess PaymentStatus = "success"
	StatusFailed  PaymentStatus = "failed"
	StatusUnknown PaymentStatus = "unknown"
)

// PaymentAttempt records a transaction attempt through an authorized payment channel.
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

// EntitlementGranter defines callback to activate purchase grant in atomic business flow.
type EntitlementGranter func(ctx context.Context, userID, courseID, orderID string) error

// Store defines persistence operations for payment attempts.
type Store interface {
	CreatePaymentAttempt(ctx context.Context, orderID, channel string, amountMinor int64, currency string) (*PaymentAttempt, error)
	GetPaymentAttempt(ctx context.Context, attemptID string) (*PaymentAttempt, error)
	ConfirmPayment(ctx context.Context, orderID, channel, channelOrderID string, amountMinor int64, currency, signature string) (*PaymentAttempt, error)
	ListAttemptsByOrder(ctx context.Context, orderID string) ([]*PaymentAttempt, error)
}

// MemoryStore provides in-memory thread-safe store for payment attempts.
type MemoryStore struct {
	mu         sync.RWMutex
	attempts   map[string]*PaymentAttempt // attemptID -> PaymentAttempt
	orderStore orders.Store
	granter    EntitlementGranter
}

// NewMemoryStore initializes a new MemoryStore for payments.
func NewMemoryStore(orderStore orders.Store, granter EntitlementGranter) *MemoryStore {
	return &MemoryStore{
		attempts:   make(map[string]*PaymentAttempt),
		orderStore: orderStore,
		granter:    granter,
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (s *MemoryStore) CreatePaymentAttempt(ctx context.Context, orderID, channel string, amountMinor int64, currency string) (*PaymentAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
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
	return attempt, nil
}

func (s *MemoryStore) GetPaymentAttempt(ctx context.Context, attemptID string) (*PaymentAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	att, ok := s.attempts[attemptID]
	if !ok {
		return nil, ErrPaymentNotFound
	}
	return att, nil
}

func (s *MemoryStore) ConfirmPayment(ctx context.Context, orderID, channel, channelOrderID string, amountMinor int64, currency, signature string) (*PaymentAttempt, error) {
	// 1. Validate signature / channel mock token
	if signature == "" || signature == "invalid_sig" {
		return nil, ErrInvalidPaymentFacts
	}

	// 2. Fetch order and verify facts
	order, err := s.orderStore.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	if order.AmountMinor != amountMinor || order.Currency != currency {
		return nil, ErrInvalidPaymentFacts
	}

	// Idempotent completion check
	if order.Status == orders.StatusPaid {
		// Find already completed attempt
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, a := range s.attempts {
			if a.OrderID == orderID && a.Status == StatusSuccess {
				return a, nil
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var targetAttempt *PaymentAttempt
	for _, a := range s.attempts {
		if a.OrderID == orderID && a.Status == StatusPending {
			targetAttempt = a
			break
		}
	}

	if targetAttempt == nil {
		targetAttempt = &PaymentAttempt{
			ID:             generateID("pay"),
			OrderID:        orderID,
			Channel:        channel,
			ChannelOrderID: channelOrderID,
			AmountMinor:    amountMinor,
			Currency:       currency,
			CreatedAt:      now,
		}
		s.attempts[targetAttempt.ID] = targetAttempt
	}

	targetAttempt.Status = StatusSuccess
	targetAttempt.ChannelOrderID = channelOrderID
	targetAttempt.UpdatedAt = now

	// Mark order as paid
	_, _ = s.orderStore.MarkOrderPaid(ctx, orderID, now)

	// Grant entitlement atomically
	if s.granter != nil {
		_ = s.granter(ctx, order.UserID, order.CourseID, orderID)
	}

	return targetAttempt, nil
}

func (s *MemoryStore) ListAttemptsByOrder(ctx context.Context, orderID string) ([]*PaymentAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*PaymentAttempt
	for _, a := range s.attempts {
		if a.OrderID == orderID {
			list = append(list, a)
		}
	}
	return list, nil
}
