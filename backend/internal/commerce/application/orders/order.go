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

	ordersdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/orders"
)

// SaleSnapshotVerifier verifies whether a course is eligible for purchase from learning.
type SaleSnapshotVerifier func(ctx context.Context, courseID string) (*ordersdomain.SaleSnapshot, error)

// ActiveEntitlementChecker checks if a user already holds an active grant for a course.
type ActiveEntitlementChecker func(ctx context.Context, userID, courseID string) (bool, error)

// Store defines persistence operations for orders and prices.
type Store interface {
	SetCoursePrice(ctx context.Context, courseID string, amountMinor int64, currency string, expectedVersion int) (*ordersdomain.Price, error)
	GetCoursePrice(ctx context.Context, courseID string) (*ordersdomain.Price, error)
	CreateOrder(ctx context.Context, userID, courseID, idempotencyKey string) (*ordersdomain.Order, error)
	GetOrder(ctx context.Context, orderID string) (*ordersdomain.Order, error)
	ListOrders(ctx context.Context, userID string, page, limit int) ([]*ordersdomain.Order, int, error)
	CancelOrder(ctx context.Context, userID, orderID string) (*ordersdomain.Order, error)
	MarkOrderPaid(ctx context.Context, orderID string, paidAt time.Time) (*ordersdomain.Order, error)
	MarkOrderRefunded(ctx context.Context, orderID string) (*ordersdomain.Order, error)
}

func ValidateCurrency(currency string) (string, error) {
	c := strings.ToUpper(strings.TrimSpace(currency))
	if c == "" {
		return "CNY", nil
	}
	if c != "CNY" {
		return "", ordersdomain.ErrUnsupportedCurrency
	}
	return c, nil
}

func IsOnSale(saleState string) bool {
	return saleState == "on_sale" || saleState == "for_sale"
}

func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func cloneOrder(o *ordersdomain.Order) *ordersdomain.Order {
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

func clonePrice(p *ordersdomain.Price) *ordersdomain.Price {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

// MemoryStore provides a thread-safe in-memory store for orders and pricing.
type MemoryStore struct {
	mu                 sync.RWMutex
	prices             map[string]*ordersdomain.Price // courseID -> Price
	orders             map[string]*ordersdomain.Order // orderID -> Order
	idempotencyIndex   map[string]string              // "userID:idempotencyKey" -> orderID
	saleVerifier       SaleSnapshotVerifier
	entitlementChecker ActiveEntitlementChecker
}

// NewMemoryStore initializes a new MemoryStore for orders.
func NewMemoryStore(verifier SaleSnapshotVerifier, checker ActiveEntitlementChecker) *MemoryStore {
	return &MemoryStore{
		prices:             make(map[string]*ordersdomain.Price),
		orders:             make(map[string]*ordersdomain.Order),
		idempotencyIndex:   make(map[string]string),
		saleVerifier:       verifier,
		entitlementChecker: checker,
	}
}

func (s *MemoryStore) SetCoursePrice(_ context.Context, courseID string, amountMinor int64, currency string, expectedVersion int) (*ordersdomain.Price, error) {
	if amountMinor < 0 {
		return nil, ordersdomain.ErrInvalidAmount
	}
	cur, err := ValidateCurrency(currency)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.prices[courseID]
	now := time.Now().UTC()
	if !ok {
		if expectedVersion > 0 {
			return nil, ordersdomain.ErrVersionConflict
		}
		price := &ordersdomain.Price{
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
		return nil, ordersdomain.ErrVersionConflict
	}

	p.AmountMinor = amountMinor
	p.Currency = cur
	p.Version++
	p.UpdatedAt = now
	return clonePrice(p), nil
}

func (s *MemoryStore) GetCoursePrice(_ context.Context, courseID string) (*ordersdomain.Price, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.prices[courseID]
	if !ok {
		return nil, ordersdomain.ErrPriceNotFound
	}
	return clonePrice(p), nil
}

func (s *MemoryStore) CreateOrder(ctx context.Context, userID, courseID, idempotencyKey string) (*ordersdomain.Order, error) {
	idempKey := fmt.Sprintf("%s:%s", userID, idempotencyKey)

	// Fast-path idempotency check before any external call.
	s.mu.RLock()
	if existingID, ok := s.idempotencyIndex[idempKey]; ok {
		existing := s.orders[existingID]
		s.mu.RUnlock()
		if existing.CourseID != courseID {
			return nil, ordersdomain.ErrIdempotencyMismatch
		}
		return cloneOrder(existing), nil
	}
	s.mu.RUnlock()

	// Verify course sale snapshot outside lock so network calls never block store mutex.
	var courseTitle string
	if s.saleVerifier != nil {
		snap, err := s.saleVerifier(ctx, courseID)
		if err != nil {
			if errors.Is(err, ordersdomain.ErrCourseNotFound) {
				return nil, ordersdomain.ErrCourseNotFound
			}
			return nil, fmt.Errorf("%w: %v", ordersdomain.ErrUpstreamUnavailable, err)
		}
		if snap == nil {
			return nil, ordersdomain.ErrUpstreamUnavailable
		}
		if !IsOnSale(snap.SaleState) {
			return nil, ordersdomain.ErrCourseNotForSale
		}
		if snap.IsFree {
			return nil, ordersdomain.ErrFreeCourseOrderNotAllowed
		}
		courseTitle = snap.Title
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Re-check idempotency under write lock.
	if existingID, ok := s.idempotencyIndex[idempKey]; ok {
		existing := s.orders[existingID]
		if existing.CourseID != courseID {
			return nil, ordersdomain.ErrIdempotencyMismatch
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
			return nil, ordersdomain.ErrAlreadyEntitled
		}
	}

	// Retrieve authoritative price from commerce.
	price, ok := s.prices[courseID]
	if !ok {
		return nil, ordersdomain.ErrPriceNotFound
	}
	if price.AmountMinor <= 0 {
		return nil, ordersdomain.ErrFreeCourseOrderNotAllowed
	}

	now := time.Now().UTC()
	orderID := NewID("ord")
	order := &ordersdomain.Order{
		ID:             orderID,
		UserID:         userID,
		CourseID:       courseID,
		CourseTitle:    courseTitle,
		AmountMinor:    price.AmountMinor,
		Currency:       price.Currency,
		Status:         ordersdomain.StatusPendingPayment,
		IdempotencyKey: idempotencyKey,
		PolicyVersion:  "v1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.orders[orderID] = order
	s.idempotencyIndex[idempKey] = orderID
	return cloneOrder(order), nil
}

func (s *MemoryStore) GetOrder(_ context.Context, orderID string) (*ordersdomain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, ok := s.orders[orderID]
	if !ok {
		return nil, ordersdomain.ErrOrderNotFound
	}
	return cloneOrder(order), nil
}

func (s *MemoryStore) ListOrders(_ context.Context, userID string, page, limit int) ([]*ordersdomain.Order, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var userOrders []*ordersdomain.Order
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
		return []*ordersdomain.Order{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	return userOrders[start:end], total, nil
}

func (s *MemoryStore) CancelOrder(_ context.Context, userID, orderID string) (*ordersdomain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[orderID]
	if !ok {
		return nil, ordersdomain.ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, ordersdomain.ErrPermissionDenied
	}
	if order.Status != ordersdomain.StatusPendingPayment {
		return nil, ordersdomain.ErrOrderNotPending
	}

	order.Status = ordersdomain.StatusCancelled
	order.UpdatedAt = time.Now().UTC()
	return cloneOrder(order), nil
}

func (s *MemoryStore) MarkOrderPaid(_ context.Context, orderID string, paidAt time.Time) (*ordersdomain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[orderID]
	if !ok {
		return nil, ordersdomain.ErrOrderNotFound
	}
	if order.Status != ordersdomain.StatusPendingPayment {
		return nil, ordersdomain.ErrOrderNotPending
	}

	t := paidAt.UTC()
	order.Status = ordersdomain.StatusPaid
	order.PaidAt = &t
	order.UpdatedAt = time.Now().UTC()
	return cloneOrder(order), nil
}

func (s *MemoryStore) MarkOrderRefunded(_ context.Context, orderID string) (*ordersdomain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[orderID]
	if !ok {
		return nil, ordersdomain.ErrOrderNotFound
	}
	if order.Status == ordersdomain.StatusRefunded {
		return cloneOrder(order), nil
	}
	if order.Status != ordersdomain.StatusPaid {
		return nil, ordersdomain.ErrOrderNotPaid
	}

	order.Status = ordersdomain.StatusRefunded
	order.UpdatedAt = time.Now().UTC()
	return cloneOrder(order), nil
}

// SetTimestampsForTest adjusts PaidAt and CreatedAt on an in-memory order for deterministic window tests.
func (s *MemoryStore) SetTimestampsForTest(orderID string, createdAt time.Time, paidAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderID]
	if !ok {
		return ordersdomain.ErrOrderNotFound
	}
	order.CreatedAt = createdAt.UTC()
	if paidAt != nil {
		t := paidAt.UTC()
		order.PaidAt = &t
	}
	return nil
}
