package orders

// SetPriceRequest defines the payload to set course price.
type SetPriceRequest struct {
	AmountMinor     *int64 `json:"amountMinor" binding:"required"`
	Currency        string `json:"currency"`
	ExpectedVersion int    `json:"expectedVersion"`
}

// CreateOrderRequest defines the payload to initiate a course purchase order.
type CreateOrderRequest struct {
	CourseID       string `json:"courseId" binding:"required"`
	IdempotencyKey string `json:"idempotencyKey" binding:"required"`
}
