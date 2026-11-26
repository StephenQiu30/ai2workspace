package payments

import (
	"time"
)

// CreatePaymentAttemptRequest defines payload to start paying for an order.
type CreatePaymentAttemptRequest struct {
	Channel        string `json:"channel" binding:"required"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

// PaymentAttemptResponse represents a serialized payment attempt.
type PaymentAttemptResponse struct {
	ID             string    `json:"id"`
	OrderID        string    `json:"orderId"`
	Channel        string    `json:"channel"`
	ChannelOrderID string    `json:"channelOrderId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	Status         string    `json:"status"`
	PaymentAction  string    `json:"paymentAction,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// PaymentStatusResponse represents reconciled order and payment status.
type PaymentStatusResponse struct {
	OrderID       string                  `json:"orderId"`
	OrderStatus   string                  `json:"orderStatus"`
	PaymentStatus string                  `json:"paymentStatus"`
	LatestAttempt *PaymentAttemptResponse `json:"latestAttempt,omitempty"`
	Recovered     bool                    `json:"recovered"`
	Message       string                  `json:"message,omitempty"`
	CheckedAt     time.Time               `json:"checkedAt"`
}

// WebhookConfirmationRequest represents payload received from payment gateway webhook.
type WebhookConfirmationRequest struct {
	OrderID        string `json:"orderId" binding:"required"`
	Channel        string `json:"channel" binding:"required"`
	ChannelOrderID string `json:"channelOrderId" binding:"required"`
	AmountMinor    int64  `json:"amountMinor" binding:"required"`
	Currency       string `json:"currency" binding:"required"`
	Signature      string `json:"signature"`
}
