package refunds

import (
	"time"
)

// ApplyRefundRequest defines payload to request a refund for a paid order.
type ApplyRefundRequest struct {
	Reason        string `json:"reason" binding:"required"`
	ExceptionType string `json:"exceptionType,omitempty"` // quality_issue, delivery_unavailable, duplicate_charge, legal_entitlement
}

// CancelRefundRequest defines optional optimistic lock payload for cancelling a pending refund.
type CancelRefundRequest struct {
	ExpectedVersion int `json:"expectedVersion"`
}

// ReviewRefundRequest defines admin decision payload for a refund application.
type ReviewRefundRequest struct {
	Decision        string `json:"decision" binding:"required"` // "approved" or "rejected"
	Reason          string `json:"reason"`
	ExpectedVersion int    `json:"expectedVersion"`
}

// ChannelConfirmRefundRequest defines channel callback or admin confirmation of refund execution result.
type ChannelConfirmRefundRequest struct {
	Status          string `json:"status" binding:"required"` // "success", "failed", "unknown"
	ChannelRefundID string `json:"channelRefundId,omitempty"`
	FailureReason   string `json:"failureReason,omitempty"`
	ExpectedVersion int    `json:"expectedVersion"`
	Signature       string `json:"signature,omitempty"`
}

// RefundResponse represents a serialized refund application.
type RefundResponse struct {
	ID                   string     `json:"id"`
	OrderID              string     `json:"orderId"`
	UserID               string     `json:"userId"`
	Reason               string     `json:"reason"`
	ExceptionType        string     `json:"exceptionType,omitempty"`
	IsOutOfWindow        bool       `json:"isOutOfWindow"`
	AmountMinor          int64      `json:"amountMinor"`
	ApplicationState     string     `json:"applicationState"`
	ChannelState         string     `json:"channelState"`
	ChannelRefundID      string     `json:"channelRefundId,omitempty"`
	ChannelFailureReason string     `json:"channelFailureReason,omitempty"`
	ReviewerID           string     `json:"reviewerId,omitempty"`
	ReviewedAt           *time.Time `json:"reviewedAt,omitempty"`
	RejectionReason      string     `json:"rejectionReason,omitempty"`
	Version              int        `json:"version"`
	FirstAvailableAt     *time.Time `json:"firstAvailableAt,omitempty"`
	Deadline             time.Time  `json:"deadline"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

// RefundListResponse represents a paginated refund list for admin console.
type RefundListResponse struct {
	Items []RefundResponse `json:"items"`
	Total int              `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}
