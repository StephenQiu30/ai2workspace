package orders

import (
	"errors"
	"time"
)

var (
	ErrOrderNotFound             = errors.New("order not found")
	ErrPriceNotFound             = errors.New("course price not configured")
	ErrCourseNotFound            = errors.New("course not found")
	ErrCourseNotForSale          = errors.New("course is not currently on sale")
	ErrFreeCourseOrderNotAllowed = errors.New("free course does not require a paid order")
	ErrAlreadyEntitled           = errors.New("user already has active entitlement for this course")
	ErrOrderNotPending           = errors.New("order is not in pending_payment state")
	ErrOrderNotPaid              = errors.New("order is not in paid state")
	ErrInvalidAmount             = errors.New("amount must be non-negative")
	ErrUnsupportedCurrency       = errors.New("only CNY currency is supported")
	ErrVersionConflict           = errors.New("version conflict: stale price update")
	ErrIdempotencyMismatch       = errors.New("idempotency key reused with different parameters")
	ErrPermissionDenied          = errors.New("permission denied")
	ErrUpstreamUnavailable       = errors.New("upstream course service unavailable")
)

// OrderStatus defines the lifecycle status of an order.
type OrderStatus string

const (
	StatusPendingPayment OrderStatus = "pending_payment"
	StatusPaid           OrderStatus = "paid"
	StatusRefunded       OrderStatus = "refunded"
	StatusCancelled      OrderStatus = "cancelled"
)

// Price represents the commercial price of a course in minor currency units (fen).
type Price struct {
	CourseID    string    `json:"courseId"`
	AmountMinor int64     `json:"amountMinor"`
	Currency    string    `json:"currency"`
	Version     int       `json:"version"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Order represents a purchase transaction for a single course.
type Order struct {
	ID             string      `json:"id"`
	UserID         string      `json:"userId"`
	CourseID       string      `json:"courseId"`
	CourseTitle    string      `json:"courseTitle"`
	AmountMinor    int64       `json:"amountMinor"`
	Currency       string      `json:"currency"`
	Status         OrderStatus `json:"status"`
	IdempotencyKey string      `json:"idempotencyKey"`
	PolicyVersion  string      `json:"policyVersion"`
	PaidAt         *time.Time  `json:"paidAt,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

// SaleSnapshot captures immutable course metadata verified from learning before order creation.
type SaleSnapshot struct {
	CourseID         string
	Title            string
	SaleState        string
	Deliverable      bool
	IsFree           bool
	PublishedVersion int
}
