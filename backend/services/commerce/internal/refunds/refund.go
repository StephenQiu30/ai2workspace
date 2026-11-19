// Package refunds manages 7x24h refund windows, admin reviews, and entitlement revocation.
package refunds

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/entitlements"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
)

var (
	ErrRefundNotFound      = errors.New("refund application not found")
	ErrOrderNotEligible    = errors.New("order is not paid or ineligible for refund")
	ErrRefundWindowExpired = errors.New("refund window expired (must be within 7x24h of course delivery)")
	ErrRefundNotPending    = errors.New("refund application is not in pending state")
	ErrVersionConflict     = errors.New("version conflict: stale refund update")
	ErrPermissionDenied    = errors.New("permission denied")
)

const RefundWindowDuration = 7 * 24 * time.Hour

// ApplicationState indicates admin approval state.
type ApplicationState string

const (
	StatePending   ApplicationState = "pending"
	StateApproved  ApplicationState = "approved"
	StateRejected  ApplicationState = "rejected"
	StateCancelled ApplicationState = "cancelled"
)

// ChannelState indicates gateway refund execution state.
type ChannelState string

const (
	ChannelNone       ChannelState = "none"
	ChannelSubmitting ChannelState = "submitting"
	ChannelSuccess    ChannelState = "success"
	ChannelFailed     ChannelState = "failed"
)

// RefundApplication records student request to return order and funds.
type RefundApplication struct {
	ID               string           `json:"id"`
	OrderID          string           `json:"orderId"`
	UserID           string           `json:"userId"`
	Reason           string           `json:"reason"`
	AmountMinor      int64            `json:"amountMinor"`
	ApplicationState ApplicationState `json:"applicationState"`
	ChannelState     ChannelState     `json:"channelState"`
	RejectionReason  string           `json:"rejectionReason,omitempty"`
	Version          int              `json:"version"`
	Deadline         time.Time        `json:"deadline"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

// DeliveryEvidenceProvider retrieves first deliverable timestamp from learning service.
type DeliveryEvidenceProvider func(ctx context.Context, courseID string, notBefore time.Time) (firstDeliverableAt *time.Time, err error)

// Store defines persistence operations for refunds.
type Store interface {
	ApplyRefund(ctx context.Context, userID, orderID, reason string) (*RefundApplication, error)
	ReviewRefund(ctx context.Context, adminID, applicationID string, approved bool, rejectionReason string, expectedVersion int) (*RefundApplication, error)
	GetRefund(ctx context.Context, applicationID string) (*RefundApplication, error)
	ListRefunds(ctx context.Context, page, limit int) ([]*RefundApplication, int, error)
}

// MemoryStore provides thread-safe in-memory store for refunds.
type MemoryStore struct {
	mu               sync.RWMutex
	refunds          map[string]*RefundApplication
	orderStore       orders.Store
	entitlementStore entitlements.Store
	evidenceProvider DeliveryEvidenceProvider
}

// NewMemoryStore initializes a new MemoryStore for refunds.
func NewMemoryStore(orderStore orders.Store, entStore entitlements.Store, evidenceProvider DeliveryEvidenceProvider) *MemoryStore {
	return &MemoryStore{
		refunds:          make(map[string]*RefundApplication),
		orderStore:       orderStore,
		entitlementStore: entStore,
		evidenceProvider: evidenceProvider,
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (s *MemoryStore) ApplyRefund(ctx context.Context, userID, orderID, reason string) (*RefundApplication, error) {
	// 1. Fetch order
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

	// 2. Determine refund window start from delivery evidence
	now := time.Now()
	startTime := order.CreatedAt
	if order.PaidAt != nil {
		startTime = *order.PaidAt
	}

	if s.evidenceProvider != nil {
		firstDeliverable, err := s.evidenceProvider(ctx, order.CourseID, startTime)
		if err == nil && firstDeliverable != nil {
			startTime = *firstDeliverable
		}
	}

	deadline := startTime.Add(RefundWindowDuration)
	if now.After(deadline) {
		return nil, ErrRefundWindowExpired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already applied
	for _, r := range s.refunds {
		if r.OrderID == orderID && (r.ApplicationState == StatePending || r.ApplicationState == StateApproved) {
			return r, nil
		}
	}

	refID := generateID("ref")
	app := &RefundApplication{
		ID:               refID,
		OrderID:          orderID,
		UserID:           userID,
		Reason:           reason,
		AmountMinor:      order.AmountMinor,
		ApplicationState: StatePending,
		ChannelState:     ChannelNone,
		Version:          1,
		Deadline:         deadline,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	s.refunds[refID] = app
	return app, nil
}

func (s *MemoryStore) ReviewRefund(ctx context.Context, adminID, applicationID string, approved bool, rejectionReason string, expectedVersion int) (*RefundApplication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.refunds[applicationID]
	if !ok {
		return nil, ErrRefundNotFound
	}
	if app.ApplicationState != StatePending {
		return nil, ErrRefundNotPending
	}
	if expectedVersion > 0 && app.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	now := time.Now()
	app.Version++
	app.UpdatedAt = now

	if approved {
		app.ApplicationState = StateApproved
		app.ChannelState = ChannelSuccess

		// Atomically revoke entitlement and mark order as refunded
		if s.entitlementStore != nil {
			_ = s.entitlementStore.RevokeEntitlement(ctx, app.OrderID)
		}
		if s.orderStore != nil {
			_, _ = s.orderStore.MarkOrderRefunded(ctx, app.OrderID)
		}
	} else {
		app.ApplicationState = StateRejected
		app.ChannelState = ChannelNone
		app.RejectionReason = rejectionReason
	}

	return app, nil
}

func (s *MemoryStore) GetRefund(ctx context.Context, applicationID string) (*RefundApplication, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.refunds[applicationID]
	if !ok {
		return nil, ErrRefundNotFound
	}
	return app, nil
}

func (s *MemoryStore) ListRefunds(ctx context.Context, page, limit int) ([]*RefundApplication, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var list []*RefundApplication
	for _, r := range s.refunds {
		list = append(list, r)
	}

	total := len(list)
	start := (page - 1) * limit
	if start >= total {
		return []*RefundApplication{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	return list[start:end], total, nil
}
