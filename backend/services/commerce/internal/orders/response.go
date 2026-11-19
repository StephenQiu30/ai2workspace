package orders

import "time"

// PriceResponse represents serialized price metadata.
type PriceResponse struct {
	CourseID    string    `json:"courseId"`
	AmountMinor int64     `json:"amountMinor"`
	Currency    string    `json:"currency"`
	Version     int       `json:"version"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// OrderResponse represents an order transaction record.
type OrderResponse struct {
	ID             string     `json:"id"`
	UserID         string     `json:"userId"`
	CourseID       string     `json:"courseId"`
	AmountMinor    int64      `json:"amountMinor"`
	Currency       string     `json:"currency"`
	Status         string     `json:"status"`
	IdempotencyKey string     `json:"idempotencyKey"`
	PolicyVersion  string     `json:"policyVersion"`
	PaidAt         *time.Time `json:"paidAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// OrderListResponse represents paginated order list.
type OrderListResponse struct {
	Items      []OrderResponse `json:"items"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	Limit      int             `json:"limit"`
	NextCursor string          `json:"nextCursor,omitempty"`
}
