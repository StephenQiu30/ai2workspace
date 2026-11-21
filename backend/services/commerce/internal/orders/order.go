// Package orders manages course pricing, order creation, and idempotent lifecycle.
package orders

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
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

// SaleSnapshotVerifier verifies whether a course is eligible for purchase from learning.
type SaleSnapshotVerifier func(ctx context.Context, courseID string) (*SaleSnapshot, error)

// ActiveEntitlementChecker checks if a user already holds an active grant for a course.
type ActiveEntitlementChecker func(ctx context.Context, userID, courseID string) (bool, error)

// Store defines persistence operations for orders and prices.
type Store interface {
	SetCoursePrice(ctx context.Context, courseID string, amountMinor int64, currency string, expectedVersion int) (*Price, error)
	GetCoursePrice(ctx context.Context, courseID string) (*Price, error)
	CreateOrder(ctx context.Context, userID, courseID, idempotencyKey string) (*Order, error)
	GetOrder(ctx context.Context, orderID string) (*Order, error)
	ListOrders(ctx context.Context, userID string, page, limit int) ([]*Order, int, error)
	CancelOrder(ctx context.Context, userID, orderID string) (*Order, error)
	MarkOrderPaid(ctx context.Context, orderID string, paidAt time.Time) (*Order, error)
	MarkOrderRefunded(ctx context.Context, orderID string) (*Order, error)
}

func validateCurrency(currency string) (string, error) {
	c := strings.ToUpper(strings.TrimSpace(currency))
	if c == "" {
		return "CNY", nil
	}
	if c != "CNY" {
		return "", ErrUnsupportedCurrency
	}
	return c, nil
}

func isOnSale(saleState string) bool {
	return saleState == "on_sale" || saleState == "for_sale"
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func cloneOrder(o *Order) *Order {
	if o == nil {
		return nil
	}
	cp := *o
	if o.PaidAt != nil {
		t := *o.PaidAt
		cp.PaidAt = &t
	}
	return &cp
}

func clonePrice(p *Price) *Price {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

// MemoryStore provides a thread-safe in-memory store for orders and pricing.
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

func (s *MemoryStore) SetCoursePrice(_ context.Context, courseID string, amountMinor int64, currency string, expectedVersion int) (*Price, error) {
	if amountMinor < 0 {
		return nil, ErrInvalidAmount
	}
	cur, err := validateCurrency(currency)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.prices[courseID]
	now := time.Now().UTC()
	if !ok {
		if expectedVersion > 0 {
			return nil, ErrVersionConflict
		}
		price := &Price{
			CourseID:    courseID,
			AmountMinor: amountMinor,
			Currency:    cur,
			Version:     1,
			UpdatedAt:   now,
		}
		s.prices[courseID] = price
		return clonePrice(price), nil
	}

	if expectedVersion > 0 && p.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	p.AmountMinor = amountMinor
	p.Currency = cur
	p.Version++
	p.UpdatedAt = now
	return clonePrice(p), nil
}

func (s *MemoryStore) GetCoursePrice(_ context.Context, courseID string) (*Price, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.prices[courseID]
	if !ok {
		return nil, ErrPriceNotFound
	}
	return clonePrice(p), nil
}

func (s *MemoryStore) CreateOrder(ctx context.Context, userID, courseID, idempotencyKey string) (*Order, error) {
	idempKey := fmt.Sprintf("%s:%s", userID, idempotencyKey)

	// Fast-path idempotency check before any external call.
	s.mu.RLock()
	if existingID, ok := s.idempotencyIndex[idempKey]; ok {
		existing := s.orders[existingID]
		s.mu.RUnlock()
		if existing.CourseID != courseID {
			return nil, ErrIdempotencyMismatch
		}
		return cloneOrder(existing), nil
	}
	s.mu.RUnlock()

	// Verify course sale snapshot outside lock so network calls never block store mutex.
	var courseTitle string
	if s.saleVerifier != nil {
		snap, err := s.saleVerifier(ctx, courseID)
		if err != nil {
			if errors.Is(err, ErrCourseNotFound) {
				return nil, ErrCourseNotFound
			}
			return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
		}
		if snap == nil {
			return nil, ErrUpstreamUnavailable
		}
		if !isOnSale(snap.SaleState) {
			return nil, ErrCourseNotForSale
		}
		if snap.IsFree {
			return nil, ErrFreeCourseOrderNotAllowed
		}
		courseTitle = snap.Title
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Re-check idempotency under write lock.
	if existingID, ok := s.idempotencyIndex[idempKey]; ok {
		existing := s.orders[existingID]
		if existing.CourseID != courseID {
			return nil, ErrIdempotencyMismatch
		}
		return cloneOrder(existing), nil
	}

	// Check if user already holds an active entitlement for this course.
	if s.entitlementChecker != nil {
		hasEntitlement, err := s.entitlementChecker(ctx, userID, courseID)
		if err != nil {
			return nil, err
		}
		if hasEntitlement {
			return nil, ErrAlreadyEntitled
		}
	}

	// Retrieve authoritative price from commerce.
	price, ok := s.prices[courseID]
	if !ok {
		return nil, ErrPriceNotFound
	}
	if price.AmountMinor <= 0 {
		return nil, ErrFreeCourseOrderNotAllowed
	}

	now := time.Now().UTC()
	orderID := generateID("ord")
	order := &Order{
		ID:             orderID,
		UserID:         userID,
		CourseID:       courseID,
		CourseTitle:    courseTitle,
		AmountMinor:    price.AmountMinor,
		Currency:       price.Currency,
		Status:         StatusPendingPayment,
		IdempotencyKey: idempotencyKey,
		PolicyVersion:  "v1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.orders[orderID] = order
	s.idempotencyIndex[idempKey] = orderID
	return cloneOrder(order), nil
}

func (s *MemoryStore) GetOrder(_ context.Context, orderID string) (*Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, ok := s.orders[orderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	return cloneOrder(order), nil
}

func (s *MemoryStore) ListOrders(_ context.Context, userID string, page, limit int) ([]*Order, int, error) {
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
			userOrders = append(userOrders, cloneOrder(o))
		}
	}

	sort.Slice(userOrders, func(i, j int) bool {
		if userOrders[i].CreatedAt.Equal(userOrders[j].CreatedAt) {
			return userOrders[i].ID > userOrders[j].ID
		}
		return userOrders[i].CreatedAt.After(userOrders[j].CreatedAt)
	})

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

func (s *MemoryStore) CancelOrder(_ context.Context, userID, orderID string) (*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[orderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, ErrPermissionDenied
	}
	if order.Status != StatusPendingPayment {
		return nil, ErrOrderNotPending
	}

	order.Status = StatusCancelled
	order.UpdatedAt = time.Now().UTC()
	return cloneOrder(order), nil
}

func (s *MemoryStore) MarkOrderPaid(_ context.Context, orderID string, paidAt time.Time) (*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[orderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	if order.Status != StatusPendingPayment {
		return nil, ErrOrderNotPending
	}

	t := paidAt.UTC()
	order.Status = StatusPaid
	order.PaidAt = &t
	order.UpdatedAt = time.Now().UTC()
	return cloneOrder(order), nil
}

func (s *MemoryStore) MarkOrderRefunded(_ context.Context, orderID string) (*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[orderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	if order.Status == StatusRefunded {
		return cloneOrder(order), nil
	}
	if order.Status != StatusPaid {
		return nil, ErrOrderNotPaid
	}

	order.Status = StatusRefunded
	order.UpdatedAt = time.Now().UTC()
	return cloneOrder(order), nil
}

// SetTimestampsForTest adjusts PaidAt and CreatedAt on an in-memory order for deterministic window tests.
func (s *MemoryStore) SetTimestampsForTest(orderID string, createdAt time.Time, paidAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderID]
	if !ok {
		return ErrOrderNotFound
	}
	order.CreatedAt = createdAt.UTC()
	if paidAt != nil {
		t := paidAt.UTC()
		order.PaidAt = &t
	}
	return nil
}
