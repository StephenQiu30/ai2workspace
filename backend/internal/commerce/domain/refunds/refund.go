package refunds

import (
	"errors"
	"time"
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
