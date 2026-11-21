// Package refunds manages 7x24h refund windows, exception manual review, admin decisions, and channel confirmation.
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

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/entitlements"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
)

var (
	ErrRefundNotFound              = errors.New("refund application not found")
	ErrOrderNotEligible            = errors.New("order is not paid or ineligible for refund")
	ErrRefundWindowExpired         = errors.New("refund window expired (must be within 7x24h of course availability)")
	ErrInvalidExceptionType        = errors.New("invalid refund exceptionType")
	ErrRefundNotPending            = errors.New("refund application is not in pending_review state")
	ErrRefundNotApproved           = errors.New("refund application is not in approved state")
	ErrRefundTerminalState         = errors.New("refund channel state is already terminal")
	ErrInvalidReviewDecision       = errors.New("decision must be 'approved' or 'rejected'")
	ErrInvalidChannelState         = errors.New("channel status must be 'success', 'failed', or 'unknown'")
	ErrVersionConflict             = errors.New("version conflict: stale refund update")
	ErrPermissionDenied            = errors.New("permission denied")
	ErrDeliveryEvidenceUnavailable = errors.New("delivery evidence service unavailable")
)

const RefundWindowDuration = 7 * 24 * time.Hour

// ApplicationState indicates manual review lifecycle state.
type ApplicationState string

const (
	StatePendingReview ApplicationState = "pending_review"
	StateApproved      ApplicationState = "approved"
	StateRejected      ApplicationState = "rejected"
	StateCancelled     ApplicationState = "cancelled"
)

// ChannelState indicates payment gateway refund execution state.
type ChannelState string

const (
	ChannelNone       ChannelState = "none"
	ChannelSubmitting ChannelState = "submitting"
	ChannelSuccess    ChannelState = "success"
	ChannelFailed     ChannelState = "failed"
	ChannelUnknown    ChannelState = "unknown"
)

// Allowed out-of-window exception types that qualify for manual review.
const (
	ExceptionQualityIssue        = "quality_issue"
	ExceptionDeliveryUnavailable = "delivery_unavailable"
	ExceptionDuplicateCharge     = "duplicate_charge"
	ExceptionLegalEntitlement    = "legal_entitlement"
)

// IsValidExceptionType reports whether exceptionType is a supported out-of-window reason code.
func IsValidExceptionType(exceptionType string) bool {
	switch exceptionType {
	case ExceptionQualityIssue, ExceptionDeliveryUnavailable, ExceptionDuplicateCharge, ExceptionLegalEntitlement:
		return true
	default:
		return false
	}
}

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

// RefundApplication records a student's request to refund a paid order.
type RefundApplication struct {
	ID                   string           `json:"id"`
	OrderID              string           `json:"orderId"`
	UserID               string           `json:"userId"`
	Reason               string           `json:"reason"`
	ExceptionType        string           `json:"exceptionType,omitempty"`
	IsOutOfWindow        bool             `json:"isOutOfWindow"`
	AmountMinor          int64            `json:"amountMinor"`
	ApplicationState     ApplicationState `json:"applicationState"`
	ChannelState         ChannelState     `json:"channelState"`
	ChannelRefundID      string           `json:"channelRefundId,omitempty"`
	ChannelFailureReason string           `json:"channelFailureReason,omitempty"`
	ReviewerID           string           `json:"reviewerId,omitempty"`
	ReviewedAt           *time.Time       `json:"reviewedAt,omitempty"`
	RejectionReason      string           `json:"rejectionReason,omitempty"`
	Version              int              `json:"version"`
	FirstAvailableAt     *time.Time       `json:"firstAvailableAt,omitempty"`
	Deadline             time.Time        `json:"deadline"`
	CreatedAt            time.Time        `json:"createdAt"`
	UpdatedAt            time.Time        `json:"updatedAt"`
}

// DeliveryEvidenceProvider retrieves the first deliverable timestamp from learning service.
type DeliveryEvidenceProvider func(ctx context.Context, courseID string, notBefore time.Time) (firstDeliverableAt *time.Time, err error)

// Notifier delivers deduplicated refund notifications to community. Failures never roll back refund facts.
type Notifier func(ctx context.Context, eventID, recipientID, notifType, title, summary string) error

// Store defines persistence operations for refund applications.
type Store interface {
	ApplyRefund(ctx context.Context, userID, orderID, reason, exceptionType string) (*RefundApplication, error)
	CancelRefund(ctx context.Context, userID, applicationID string, expectedVersion int) (*RefundApplication, error)
	ReviewRefund(ctx context.Context, adminID, applicationID, decision, rejectionReason string, expectedVersion int) (*RefundApplication, error)
	ConfirmChannelRefund(ctx context.Context, applicationID string, status ChannelState, channelRefundID, failureReason string, expectedVersion int) (*RefundApplication, error)
	GetRefund(ctx context.Context, applicationID string) (*RefundApplication, error)
	ListRefunds(ctx context.Context, page, limit int) ([]*RefundApplication, int, error)
	ListRefundsByOrder(ctx context.Context, orderID string) ([]*RefundApplication, error)
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func cloneRefund(r *RefundApplication) *RefundApplication {
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

func computeRefundWindow(ctx context.Context, order *orders.Order, evidenceProvider DeliveryEvidenceProvider, now time.Time) (firstAvailableAt *time.Time, deadline time.Time, outOfWindow bool, err error) {
	paidAt := order.CreatedAt
	if order.PaidAt != nil {
		paidAt = *order.PaidAt
	}

	if evidenceProvider != nil {
		firstDeliverable, evErr := evidenceProvider(ctx, order.CourseID, paidAt)
		if evErr != nil {
			return nil, time.Time{}, false, fmt.Errorf("%w: %v", ErrDeliveryEvidenceUnavailable, evErr)
		}
		if firstDeliverable == nil {
			// Verified as not yet delivered: keep refund entry open without fabricating a past start time.
			return nil, now.Add(RefundWindowDuration), false, nil
		}
		windowStart := paidAt
		if firstDeliverable.After(paidAt) {
			windowStart = *firstDeliverable
		}
		t := windowStart.UTC()
		dl := t.Add(RefundWindowDuration)
		return &t, dl, now.After(dl), nil
	}

	t := paidAt.UTC()
	dl := t.Add(RefundWindowDuration)
	return &t, dl, now.After(dl), nil
}

func sendRefundSucceededNotification(ctx context.Context, notifier Notifier, app *RefundApplication) {
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
	refunds          map[string]*RefundApplication
	orderStore       orders.Store
	entitlementStore entitlements.Store
	evidenceProvider DeliveryEvidenceProvider
	notifier         Notifier
}

// NewMemoryStore initializes a new MemoryStore for refunds.
func NewMemoryStore(orderStore orders.Store, entStore entitlements.Store, evidenceProvider DeliveryEvidenceProvider, notifier Notifier) *MemoryStore {
	return &MemoryStore{
		refunds:          make(map[string]*RefundApplication),
		orderStore:       orderStore,
		entitlementStore: entStore,
		evidenceProvider: evidenceProvider,
		notifier:         notifier,
	}
}

func (s *MemoryStore) ApplyRefund(ctx context.Context, userID, orderID, reason, exceptionType string) (*RefundApplication, error) {
	reason = strings.TrimSpace(reason)
	exceptionType = strings.TrimSpace(exceptionType)

	if exceptionType != "" && !IsValidExceptionType(exceptionType) {
		return nil, ErrInvalidExceptionType
	}

	// 1. Fetch and validate order outside lock.
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

	// 2. Compute refund window using delivery evidence (never silently shorten on upstream failure).
	now := time.Now().UTC()
	firstAvailableAt, deadline, isOutOfWindow, err := computeRefundWindow(ctx, order, s.evidenceProvider, now)
	if err != nil {
		return nil, err
	}

	if isOutOfWindow {
		if exceptionType == "" {
			return nil, ErrRefundWindowExpired
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Reuse active pending/approved refund application for this order to prevent duplicate refund requests.
	for _, r := range s.refunds {
		if r.OrderID == orderID && (r.ApplicationState == StatePendingReview || r.ApplicationState == StateApproved) {
			return cloneRefund(r), nil
		}
	}

	refID := generateID("ref")
	app := &RefundApplication{
		ID:               refID,
		OrderID:          orderID,
		UserID:           userID,
		Reason:           reason,
		ExceptionType:    exceptionType,
		IsOutOfWindow:    isOutOfWindow,
		AmountMinor:      order.AmountMinor,
		ApplicationState: StatePendingReview,
		ChannelState:     ChannelNone,
		Version:          1,
		FirstAvailableAt: firstAvailableAt,
		Deadline:         deadline,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	s.refunds[refID] = app
	return cloneRefund(app), nil
}

func (s *MemoryStore) CancelRefund(_ context.Context, userID, applicationID string, expectedVersion int) (*RefundApplication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.refunds[applicationID]
	if !ok {
		return nil, ErrRefundNotFound
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

	app.ApplicationState = StateCancelled
	app.Version++
	app.UpdatedAt = time.Now().UTC()
	return cloneRefund(app), nil
}

func (s *MemoryStore) ReviewRefund(_ context.Context, adminID, applicationID, decision, rejectionReason string, expectedVersion int) (*RefundApplication, error) {
	if decision != "approved" && decision != "rejected" {
		return nil, ErrInvalidReviewDecision
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.refunds[applicationID]
	if !ok {
		return nil, ErrRefundNotFound
	}
	if app.ApplicationState != StatePendingReview {
		return nil, ErrRefundNotPending
	}
	if expectedVersion > 0 && app.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	now := time.Now().UTC()
	app.Version++
	app.ReviewerID = adminID
	app.ReviewedAt = &now
	app.UpdatedAt = now

	if decision == "approved" {
		// Approval enters channel submitting stage; entitlement remains ACTIVE until channel confirms success.
		app.ApplicationState = StateApproved
		app.ChannelState = ChannelSubmitting
	} else {
		app.ApplicationState = StateRejected
		app.ChannelState = ChannelNone
		app.RejectionReason = strings.TrimSpace(rejectionReason)
	}

	return cloneRefund(app), nil
}

func (s *MemoryStore) ConfirmChannelRefund(ctx context.Context, applicationID string, status ChannelState, channelRefundID, failureReason string, expectedVersion int) (*RefundApplication, error) {
	if status != ChannelSuccess && status != ChannelFailed && status != ChannelUnknown {
		return nil, ErrInvalidChannelState
	}

	result, shouldNotify, err := s.confirmChannelLocked(ctx, applicationID, status, channelRefundID, failureReason, expectedVersion)
	if err != nil {
		return nil, err
	}
	if shouldNotify {
		sendRefundSucceededNotification(ctx, s.notifier, result)
	}
	return result, nil
}

func (s *MemoryStore) confirmChannelLocked(ctx context.Context, applicationID string, status ChannelState, channelRefundID, failureReason string, expectedVersion int) (*RefundApplication, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.refunds[applicationID]
	if !ok {
		return nil, false, ErrRefundNotFound
	}
	if app.ApplicationState != StateApproved {
		return nil, false, ErrRefundNotApproved
	}
	if expectedVersion > 0 && app.Version != expectedVersion {
		return nil, false, ErrVersionConflict
	}

	// Protect terminal channel success from out-of-order regression.
	if app.ChannelState == ChannelSuccess {
		if status == ChannelSuccess {
			return cloneRefund(app), false, nil
		}
		return nil, false, ErrRefundTerminalState
	}

	now := time.Now().UTC()
	if status == ChannelSuccess {
		// Only revoke entitlement for THIS specific orderID, preserving other active orders/repurchases.
		if s.entitlementStore != nil {
			if err := s.entitlementStore.RevokeEntitlement(ctx, app.OrderID); err != nil && !errors.Is(err, entitlements.ErrEntitlementNotFound) {
				return nil, false, fmt.Errorf("revoke entitlement failed: %w", err)
			}
		}
		if s.orderStore != nil {
			if _, err := s.orderStore.MarkOrderRefunded(ctx, app.OrderID); err != nil {
				return nil, false, fmt.Errorf("mark order refunded failed: %w", err)
			}
		}
		app.ChannelState = ChannelSuccess
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

func (s *MemoryStore) GetRefund(_ context.Context, applicationID string) (*RefundApplication, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.refunds[applicationID]
	if !ok {
		return nil, ErrRefundNotFound
	}
	return cloneRefund(app), nil
}

func (s *MemoryStore) ListRefunds(_ context.Context, page, limit int) ([]*RefundApplication, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	list := make([]*RefundApplication, 0, len(s.refunds))
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
		return []*RefundApplication{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	return list[start:end], total, nil
}

func (s *MemoryStore) ListRefundsByOrder(_ context.Context, orderID string) ([]*RefundApplication, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*RefundApplication, 0)
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
