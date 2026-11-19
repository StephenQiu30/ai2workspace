// Package entitlements manages access grant facts purchased by students.
package entitlements

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
	ErrEntitlementNotFound = errors.New("entitlement not found")
)

// GrantStatus defines active or revoked state of an entitlement.
type GrantStatus string

const (
	GrantActive  GrantStatus = "active"
	GrantRevoked GrantStatus = "revoked"
)

// Entitlement represents a purchased learning access grant.
type Entitlement struct {
	ID           string      `json:"id"`
	UserID       string      `json:"userId"`
	CourseID     string      `json:"courseId"`
	OrderID      string      `json:"orderId"`
	Status       GrantStatus `json:"status"`
	GrantVersion int         `json:"grantVersion"`
	ActivatedAt  time.Time   `json:"activatedAt"`
	RevokedAt    *time.Time  `json:"revokedAt,omitempty"`
}

// Store defines persistence operations for entitlements.
type Store interface {
	GrantEntitlement(ctx context.Context, userID, courseID, orderID string) (*Entitlement, error)
	CheckEntitlement(ctx context.Context, userID, courseID, purpose string) (allowed bool, grantVersion int, reason string, err error)
	RevokeEntitlement(ctx context.Context, orderID string) error
	ListUserEntitlements(ctx context.Context, userID string) ([]*Entitlement, error)
}

// MemoryStore provides in-memory thread-safe store for entitlements.
type MemoryStore struct {
	mu           sync.RWMutex
	entitlements map[string]*Entitlement // entitlementID -> Entitlement
}

// NewMemoryStore initializes a new MemoryStore for entitlements.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entitlements: make(map[string]*Entitlement),
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (s *MemoryStore) GrantEntitlement(ctx context.Context, userID, courseID, orderID string) (*Entitlement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already granted for this order
	for _, e := range s.entitlements {
		if e.OrderID == orderID {
			return e, nil
		}
	}

	now := time.Now()
	entID := generateID("ent")
	ent := &Entitlement{
		ID:           entID,
		UserID:       userID,
		CourseID:     courseID,
		OrderID:      orderID,
		Status:       GrantActive,
		GrantVersion: 1,
		ActivatedAt:  now,
	}

	s.entitlements[entID] = ent
	return ent, nil
}

func (s *MemoryStore) CheckEntitlement(ctx context.Context, userID, courseID, purpose string) (allowed bool, grantVersion int, reason string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range s.entitlements {
		if e.UserID == userID && e.CourseID == courseID && e.Status == GrantActive {
			return true, e.GrantVersion, "active_purchase_grant", nil
		}
	}

	return false, 0, "not_purchased", nil
}

func (s *MemoryStore) RevokeEntitlement(ctx context.Context, orderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, e := range s.entitlements {
		if e.OrderID == orderID && e.Status == GrantActive {
			e.Status = GrantRevoked
			e.RevokedAt = &now
			e.GrantVersion++
			return nil
		}
	}
	return ErrEntitlementNotFound
}

func (s *MemoryStore) ListUserEntitlements(ctx context.Context, userID string) ([]*Entitlement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*Entitlement
	for _, e := range s.entitlements {
		if e.UserID == userID {
			list = append(list, e)
		}
	}
	return list, nil
}
