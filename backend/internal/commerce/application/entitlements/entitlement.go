package entitlements

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	entitlementsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/entitlements"
)

// Store defines persistence operations for entitlements.
type Store interface {
	GrantEntitlement(ctx context.Context, userID, courseID, orderID string) (*entitlementsdomain.Entitlement, error)
	CheckEntitlement(ctx context.Context, userID, courseID, purpose string) (allowed bool, grantVersion int, reason string, err error)
	RevokeEntitlement(ctx context.Context, orderID string) error
	ListUserEntitlements(ctx context.Context, userID string) ([]*entitlementsdomain.Entitlement, error)
}

func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func cloneEntitlement(e *entitlementsdomain.Entitlement) *entitlementsdomain.Entitlement {
	if e == nil {
		return nil
	}
	cp := *e
	if e.RevokedAt != nil {
		t := *e.RevokedAt
		cp.RevokedAt = &t
	}
	return &cp
}

// MemoryStore provides a thread-safe in-memory store for entitlements.
type MemoryStore struct {
	mu           sync.RWMutex
	entitlements map[string]*entitlementsdomain.Entitlement // entitlementID -> Entitlement
}

// NewMemoryStore initializes a new MemoryStore for entitlements.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entitlements: make(map[string]*entitlementsdomain.Entitlement),
	}
}

func (s *MemoryStore) GrantEntitlement(_ context.Context, userID, courseID, orderID string) (*entitlementsdomain.Entitlement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range s.entitlements {
		if e.OrderID == orderID {
			return cloneEntitlement(e), nil
		}
	}

	now := time.Now().UTC()
	entID := NewID("ent")
	ent := &entitlementsdomain.Entitlement{
		ID:           entID,
		UserID:       userID,
		CourseID:     courseID,
		OrderID:      orderID,
		Status:       entitlementsdomain.GrantActive,
		GrantVersion: 1,
		ActivatedAt:  now,
	}

	s.entitlements[entID] = ent
	return cloneEntitlement(ent), nil
}

func (s *MemoryStore) CheckEntitlement(_ context.Context, userID, courseID, _ string) (allowed bool, grantVersion int, reason string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest *entitlementsdomain.Entitlement
	for _, e := range s.entitlements {
		if e.UserID == userID && e.CourseID == courseID && e.Status == entitlementsdomain.GrantActive {
			if latest == nil || e.ActivatedAt.After(latest.ActivatedAt) {
				latest = e
			}
		}
	}
	if latest != nil {
		return true, latest.GrantVersion, "active_purchase_grant", nil
	}

	return false, 0, "not_purchased", nil
}

func (s *MemoryStore) RevokeEntitlement(_ context.Context, orderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for _, e := range s.entitlements {
		if e.OrderID == orderID {
			if e.Status == entitlementsdomain.GrantRevoked {
				return nil
			}
			e.Status = entitlementsdomain.GrantRevoked
			e.RevokedAt = &now
			e.GrantVersion++
			return nil
		}
	}
	return entitlementsdomain.ErrEntitlementNotFound
}

func (s *MemoryStore) ListUserEntitlements(_ context.Context, userID string) ([]*entitlementsdomain.Entitlement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*entitlementsdomain.Entitlement, 0)
	for _, e := range s.entitlements {
		if e.UserID == userID {
			list = append(list, cloneEntitlement(e))
		}
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].ActivatedAt.Equal(list[j].ActivatedAt) {
			return list[i].ID > list[j].ID
		}
		return list[i].ActivatedAt.After(list[j].ActivatedAt)
	})

	return list, nil
}
