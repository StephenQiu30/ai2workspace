// Package orders manages course pricing, order creation, and idempotent lifecycle.
package orders

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrOrderNotFound       = errors.New("order not found")
	ErrPriceNotFound       = errors.New("course price not configured")
	ErrAlreadyEntitled     = errors.New("user already has active entitlement for this course")
	ErrOrderNotPending     = errors.New("order is not in pending state")
	ErrInvalidAmount       = errors.New("amount must be non-negative")
	ErrVersionConflict     = errors.New("version conflict: stale price update")
	ErrIdempotencyMismatch = errors.New("idempotency key reused with different parameters")
)

// OrderStatus defines the lifecycle status of an order.
type OrderStatus string

const (
	StatusPending   OrderStatus = "pending"
	StatusPaid      OrderStatus = "paid"
	StatusRefunded  OrderStatus = "refunded"
	StatusCancelled OrderStatus = "cancelled"
)

// Price represents the commercial price of a course in minor currency units (cents).
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
	AmountMinor    int64       `json:"amountMinor"`
	Currency       string      `json:"currency"`
	Status         OrderStatus `json:"status"`
	IdempotencyKey string      `json:"idempotencyKey"`
	PolicyVersion  string      `json:"policyVersion"`
	PaidAt         *time.Time  `json:"paidAt,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

// SaleSnapshotVerifier verifies whether a course is eligible for purchase from learning.
type SaleSnapshotVerifier func(courseID string) (saleState string, deliverable bool, err error)

// ActiveEntitlementChecker checks if a user already holds an active grant for a course.
type ActiveEntitlementChecker func(userID, courseID string) (bool, error)

// Store defines persistence operations for orders and prices.
type Store interface {
	SetCoursePrice(ctx context.Context, courseID string, amountMinor int64, currency string, expectedVersion int) (*Price, error)
	GetCoursePrice(ctx context.Context, courseID string) (*Price, error)
	CreateOrder(ctx context.Context, userID, courseID, idempotencyKey string) (*Order, error)
	GetOrder(ctx context.Context, orderID string) (*Order, error)
	ListOrders(ctx context.Context, userID string, page, limit int) ([]*Order, int, error)
	MarkOrderPaid(ctx context.Context, orderID string, paidAt time.Time) (*Order, error)
	MarkOrderRefunded(ctx context.Context, orderID string) (*Order, error)
}

// MemoryStore provides thread-safe in-memory store for orders and pricing.
type MemoryStore struct {
	mu                 sync.RWMutex
	prices             map[string]*Price // courseID -> Price
	orders             map[string]*Order // orderID -> Order
	idempotencyIndex   map[string]string // "userID:idempotencyKey" -> orderID
	saleVerifier       SaleSnapshotVerifier
	entitlementChecker ActiveEntitlementChecker
}

// NewMemoryStore initializes a new MemoryStore for orders.
func NewMemoryStore(verifier SaleSnapshotVerifier, checker ActiveEntitlementChecker) *MemoryStore {
	return &MemoryStore{
		prices:             make(map[string]*Price),
		orders:             make(map[string]*Order),
		idempotencyIndex:   make(map[string]string),
		saleVerifier:       verifier,
		entitlementChecker: checker,
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (s *MemoryStore) SetCoursePrice(ctx context.Context, courseID string, amountMinor int64, currency string, expectedVersion int) (*Price, error) {
	if amountMinor < 0 {
		return nil, ErrInvalidAmount
	}
	if currency == "" {
		currency = "CNY"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.prices[courseID]
	now := time.Now()
	if !ok {
		price := &Price{
			CourseID:    courseID,
			AmountMinor: amountMinor,
			Currency:    currency,
			Version:     1,
			UpdatedAt:   now,
		}
		s.prices[courseID] = price
		return price, nil
	}

	if expectedVersion > 0 && p.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	p.AmountMinor = amountMinor
	p.Currency = currency
	p.Version++
	p.UpdatedAt = now
	return p, nil
}

func (s *MemoryStore) GetCoursePrice(ctx context.Context, courseID string) (*Price, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.prices[courseID]
	if !ok {
		return nil, ErrPriceNotFound
	}
	return p, nil
}

func (s *MemoryStore) CreateOrder(ctx context.Context, userID, courseID, idempotencyKey string) (*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Check idempotency
	idempKey := fmt.Sprintf("%s:%s", userID, idempotencyKey)
	if existingID, ok := s.idempotencyIndex[idempKey]; ok {
		existing := s.orders[existingID]
		if existing.CourseID != courseID {
			return nil, ErrIdempotencyMismatch
		}
		return existing, nil
	}

	// 2. Check if user already holds active entitlement
	if s.entitlementChecker != nil {
		hasEntitlement, err := s.entitlementChecker(userID, courseID)
		if err != nil {
			return nil, err
		}
		if hasEntitlement {
			return nil, ErrAlreadyEntitled
		}
	}

	// 3. Verify sale state if verifier attached
	if s.saleVerifier != nil {
		saleState, _, err := s.saleVerifier(courseID)
		if err != nil {
			return nil, err
		}
		if saleState == "closed" {
			return nil, errors.New("course sales are closed")
		}
	}

	// 4. Retrieve course price
	price, ok := s.prices[courseID]
	if !ok {
		return nil, ErrPriceNotFound
	}

	now := time.Now()
	orderID := generateID("ord")
	order := &Order{
		ID:             orderID,
		UserID:         userID,
		CourseID:       courseID,
		AmountMinor:    price.AmountMinor,
		Currency:       price.Currency,
		Status:         StatusPending,
		IdempotencyKey: idempotencyKey,
		PolicyVersion:  "v1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.orders[orderID] = order
	s.idempotencyIndex[idempKey] = orderID
	return order, nil
}

func (s *MemoryStore) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, ok := s.orders[orderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	return order, nil
}

func (s *MemoryStore) ListOrders(ctx context.Context, userID string, page, limit int) ([]*Order, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var userOrders []*Order
	for _, o := range s.orders {
		if o.UserID == userID {
			userOrders = append(userOrders, o)
		}
	}

	total := len(userOrders)
	start := (page - 1) * limit
	if start >= total {
		return []*Order{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	return userOrders[start:end], total, nil
}

func (s *MemoryStore) MarkOrderPaid(ctx context.Context, orderID string, paidAt time.Time) (*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[orderID]
	if !ok {
		return nil, ErrOrderNotFound
	}

	order.Status = StatusPaid
	order.PaidAt = &paidAt
	order.UpdatedAt = time.Now()
	return order, nil
}

func (s *MemoryStore) MarkOrderRefunded(ctx context.Context, orderID string) (*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[orderID]
	if !ok {
		return nil, ErrOrderNotFound
	}

	order.Status = StatusRefunded
	order.UpdatedAt = time.Now()
	return order, nil
}
