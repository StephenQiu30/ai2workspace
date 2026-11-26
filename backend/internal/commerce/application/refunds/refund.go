package refunds

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

	entitlementsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/entitlements"
	ordersapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/orders"
	entitlementsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/entitlements"
	ordersdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/orders"
	refundsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/refunds"
)

// ComputeChannelConfirmSignature calculates the canonical HMAC-SHA256 hex digest for refund channel confirmation.
func ComputeChannelConfirmSignature(secret, refundID, channelRefundID, status string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	payload := fmt.Sprintf("%s|%s|%s", refundID, channelRefundID, status)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyChannelConfirmSignature checks an HMAC-SHA256 signature for channel refund callbacks.
func VerifyChannelConfirmSignature(secret, refundID, channelRefundID, status, provided string) bool {
	if secret == "" || strings.TrimSpace(provided) == "" {
		return false
	}
	expected := ComputeChannelConfirmSignature(secret, refundID, channelRefundID, status)
	return hmac.Equal([]byte(strings.ToLower(strings.TrimSpace(provided))), []byte(expected))
}

// DeliveryEvidenceProvider retrieves the first deliverable timestamp from learning service.
type DeliveryEvidenceProvider func(ctx context.Context, courseID string, notBefore time.Time) (firstDeliverableAt *time.Time, err error)

// Notifier delivers deduplicated refund notifications to community. Failures never roll back refund facts.
type Notifier func(ctx context.Context, eventID, recipientID, notifType, title, summary string) error

// Store defines persistence operations for refund applications.
type Store interface {
	ApplyRefund(ctx context.Context, userID, orderID, reason, exceptionType string) (*refundsdomain.RefundApplication, error)
	CancelRefund(ctx context.Context, userID, applicationID string, expectedVersion int) (*refundsdomain.RefundApplication, error)
	ReviewRefund(ctx context.Context, adminID, applicationID, decision, rejectionReason string, expectedVersion int) (*refundsdomain.RefundApplication, error)
	ConfirmChannelRefund(ctx context.Context, applicationID string, status refundsdomain.ChannelState, channelRefundID, failureReason string, expectedVersion int) (*refundsdomain.RefundApplication, error)
	GetRefund(ctx context.Context, applicationID string) (*refundsdomain.RefundApplication, error)
	ListRefunds(ctx context.Context, page, limit int) ([]*refundsdomain.RefundApplication, int, error)
	ListRefundsByOrder(ctx context.Context, orderID string) ([]*refundsdomain.RefundApplication, error)
}

func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func cloneRefund(r *refundsdomain.RefundApplication) *refundsdomain.RefundApplication {
	if r == nil {
		return nil
	}
	cp := *r
	if r.ReviewedAt != nil {
		t := *r.ReviewedAt
		cp.ReviewedAt = &t
	}
	if r.FirstAvailableAt != nil {
		t := *r.FirstAvailableAt
		cp.FirstAvailableAt = &t
	}
	return &cp
}

func ComputeRefundWindow(ctx context.Context, order *ordersdomain.Order, evidenceProvider DeliveryEvidenceProvider, now time.Time) (firstAvailableAt *time.Time, deadline time.Time, outOfWindow bool, err error) {
	paidAt := order.CreatedAt
	if order.PaidAt != nil {
		paidAt = *order.PaidAt
	}

	if evidenceProvider != nil {
		firstDeliverable, evErr := evidenceProvider(ctx, order.CourseID, paidAt)
		if evErr != nil {
			return nil, time.Time{}, false, fmt.Errorf("%w: %v", refundsdomain.ErrDeliveryEvidenceUnavailable, evErr)
		}
		if firstDeliverable == nil {
			// Verified as not yet delivered: keep refund entry open without fabricating a past start time.
			return nil, now.Add(refundsdomain.RefundWindowDuration), false, nil
		}
		windowStart := paidAt
		if firstDeliverable.After(paidAt) {
			windowStart = *firstDeliverable
		}
		t := windowStart.UTC()
		dl := t.Add(refundsdomain.RefundWindowDuration)
		return &t, dl, now.After(dl), nil
	}

	t := paidAt.UTC()
	dl := t.Add(refundsdomain.RefundWindowDuration)
	return &t, dl, now.After(dl), nil
}

func SendRefundSucceededNotification(ctx context.Context, notifier Notifier, app *refundsdomain.RefundApplication) {
	if notifier == nil || app == nil {
		return
	}
	eventID := fmt.Sprintf("commerce:refund_succeeded:%s", app.ID)
	title := "课程退款已完成"
	summary := fmt.Sprintf("订单 %s 的全额原路退款（申请单 %s）已确认成功，对应课程权益已撤销。", app.OrderID, app.ID)
	notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := notifier(notifyCtx, eventID, app.UserID, "refund_succeeded", title, summary); err != nil {
		slog.Warn("failed to deliver refund notification", "refundId", app.ID, "orderId", app.OrderID, "eventId", eventID, "error", err)
	}
}

// MemoryStore provides a thread-safe in-memory store for refunds.
type MemoryStore struct {
	mu               sync.RWMutex
	refunds          map[string]*refundsdomain.RefundApplication
	orderStore       ordersapp.Store
	entitlementStore entitlementsapp.Store
	evidenceProvider DeliveryEvidenceProvider
	notifier         Notifier
}

// NewMemoryStore initializes a new MemoryStore for refunds.
func NewMemoryStore(orderStore ordersapp.Store, entStore entitlementsapp.Store, evidenceProvider DeliveryEvidenceProvider, notifier Notifier) *MemoryStore {
	return &MemoryStore{
		refunds:          make(map[string]*refundsdomain.RefundApplication),
		orderStore:       orderStore,
		entitlementStore: entStore,
		evidenceProvider: evidenceProvider,
		notifier:         notifier,
	}
}

func (s *MemoryStore) ApplyRefund(ctx context.Context, userID, orderID, reason, exceptionType string) (*refundsdomain.RefundApplication, error) {
	reason = strings.TrimSpace(reason)
	exceptionType = strings.TrimSpace(exceptionType)

	if exceptionType != "" && !refundsdomain.IsValidExceptionType(exceptionType) {
		return nil, refundsdomain.ErrInvalidExceptionType
	}

	// 1. Fetch and validate order outside lock.
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

	// 2. Compute refund window using delivery evidence (never silently shorten on upstream failure).
	now := time.Now().UTC()
	firstAvailableAt, deadline, isOutOfWindow, err := ComputeRefundWindow(ctx, order, s.evidenceProvider, now)
	if err != nil {
		return nil, err
	}

	if isOutOfWindow {
		if exceptionType == "" {
			return nil, refundsdomain.ErrRefundWindowExpired
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Reuse active pending/approved refund application for this order to prevent duplicate refund requests.
	for _, r := range s.refunds {
		if r.OrderID == orderID && (r.ApplicationState == refundsdomain.StatePendingReview || r.ApplicationState == refundsdomain.StateApproved) {
			return cloneRefund(r), nil
		}
	}

	refID := NewID("ref")
	app := &refundsdomain.RefundApplication{
		ID:               refID,
		OrderID:          orderID,
		UserID:           userID,
		Reason:           reason,
		ExceptionType:    exceptionType,
		IsOutOfWindow:    isOutOfWindow,
		AmountMinor:      order.AmountMinor,
		ApplicationState: refundsdomain.StatePendingReview,
		ChannelState:     refundsdomain.ChannelNone,
		Version:          1,
		FirstAvailableAt: firstAvailableAt,
		Deadline:         deadline,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	s.refunds[refID] = app
	return cloneRefund(app), nil
}

func (s *MemoryStore) CancelRefund(_ context.Context, userID, applicationID string, expectedVersion int) (*refundsdomain.RefundApplication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.refunds[applicationID]
	if !ok {
		return nil, refundsdomain.ErrRefundNotFound
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

	app.ApplicationState = refundsdomain.StateCancelled
	app.Version++
	app.UpdatedAt = time.Now().UTC()
	return cloneRefund(app), nil
}

func (s *MemoryStore) ReviewRefund(_ context.Context, adminID, applicationID, decision, rejectionReason string, expectedVersion int) (*refundsdomain.RefundApplication, error) {
	if decision != "approved" && decision != "rejected" {
		return nil, refundsdomain.ErrInvalidReviewDecision
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.refunds[applicationID]
	if !ok {
		return nil, refundsdomain.ErrRefundNotFound
	}
	if app.ApplicationState != refundsdomain.StatePendingReview {
		return nil, refundsdomain.ErrRefundNotPending
	}
	if expectedVersion > 0 && app.Version != expectedVersion {
		return nil, refundsdomain.ErrVersionConflict
	}

	now := time.Now().UTC()
	app.Version++
	app.ReviewerID = adminID
	app.ReviewedAt = &now
	app.UpdatedAt = now

	if decision == "approved" {
		// Approval enters channel submitting stage; entitlement remains ACTIVE until channel confirms success.
		app.ApplicationState = refundsdomain.StateApproved
		app.ChannelState = refundsdomain.ChannelSubmitting
	} else {
		app.ApplicationState = refundsdomain.StateRejected
		app.ChannelState = refundsdomain.ChannelNone
		app.RejectionReason = strings.TrimSpace(rejectionReason)
	}

	return cloneRefund(app), nil
}

func (s *MemoryStore) ConfirmChannelRefund(ctx context.Context, applicationID string, status refundsdomain.ChannelState, channelRefundID, failureReason string, expectedVersion int) (*refundsdomain.RefundApplication, error) {
	if status != refundsdomain.ChannelSuccess && status != refundsdomain.ChannelFailed && status != refundsdomain.ChannelUnknown {
		return nil, refundsdomain.ErrInvalidChannelState
	}

	result, shouldNotify, err := s.confirmChannelLocked(ctx, applicationID, status, channelRefundID, failureReason, expectedVersion)
	if err != nil {
		return nil, err
	}
	if shouldNotify {
		SendRefundSucceededNotification(ctx, s.notifier, result)
	}
	return result, nil
}

func (s *MemoryStore) confirmChannelLocked(ctx context.Context, applicationID string, status refundsdomain.ChannelState, channelRefundID, failureReason string, expectedVersion int) (*refundsdomain.RefundApplication, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.refunds[applicationID]
	if !ok {
		return nil, false, refundsdomain.ErrRefundNotFound
	}
	if app.ApplicationState != refundsdomain.StateApproved {
		return nil, false, refundsdomain.ErrRefundNotApproved
	}
	if expectedVersion > 0 && app.Version != expectedVersion {
		return nil, false, refundsdomain.ErrVersionConflict
	}

	// Protect terminal channel success from out-of-order regression.
	if app.ChannelState == refundsdomain.ChannelSuccess {
		if status == refundsdomain.ChannelSuccess {
			return cloneRefund(app), false, nil
		}
		return nil, false, refundsdomain.ErrRefundTerminalState
	}

	now := time.Now().UTC()
	if status == refundsdomain.ChannelSuccess {
		// Only revoke entitlement for THIS specific orderID, preserving other active orders/repurchases.
		if s.entitlementStore != nil {
			if err := s.entitlementStore.RevokeEntitlement(ctx, app.OrderID); err != nil && !errors.Is(err, entitlementsdomain.ErrEntitlementNotFound) {
				return nil, false, fmt.Errorf("revoke entitlement failed: %w", err)
			}
		}
		if s.orderStore != nil {
			if _, err := s.orderStore.MarkOrderRefunded(ctx, app.OrderID); err != nil {
				return nil, false, fmt.Errorf("mark order refunded failed: %w", err)
			}
		}
		app.ChannelState = refundsdomain.ChannelSuccess
		if channelRefundID != "" {
			app.ChannelRefundID = channelRefundID
		}
		app.ChannelFailureReason = ""
		app.Version++
		app.UpdatedAt = now
		return cloneRefund(app), true, nil
	}

	// Failed or unknown: do NOT revoke entitlement or mark order refunded.
	app.ChannelState = status
	if channelRefundID != "" {
		app.ChannelRefundID = channelRefundID
	}
	app.ChannelFailureReason = strings.TrimSpace(failureReason)
	app.Version++
	app.UpdatedAt = now
	return cloneRefund(app), false, nil
}

func (s *MemoryStore) GetRefund(_ context.Context, applicationID string) (*refundsdomain.RefundApplication, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.refunds[applicationID]
	if !ok {
		return nil, refundsdomain.ErrRefundNotFound
	}
	return cloneRefund(app), nil
}

func (s *MemoryStore) ListRefunds(_ context.Context, page, limit int) ([]*refundsdomain.RefundApplication, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	list := make([]*refundsdomain.RefundApplication, 0, len(s.refunds))
	for _, r := range s.refunds {
		list = append(list, cloneRefund(r))
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID > list[j].ID
		}
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	total := len(list)
	start := (page - 1) * limit
	if start >= total {
		return []*refundsdomain.RefundApplication{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	return list[start:end], total, nil
}

func (s *MemoryStore) ListRefundsByOrder(_ context.Context, orderID string) ([]*refundsdomain.RefundApplication, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*refundsdomain.RefundApplication, 0)
	for _, r := range s.refunds {
		if r.OrderID == orderID {
			list = append(list, cloneRefund(r))
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
