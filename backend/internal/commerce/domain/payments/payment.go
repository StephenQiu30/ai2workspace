package payments

import (
	"errors"
	"time"
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
