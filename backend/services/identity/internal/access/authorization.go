// Package access manages admin authorizations, instructor invitations, and internal service identity resolution.
package access

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvitationNotFound   = errors.New("invitation not found")
	ErrInvitationNotPending = errors.New("invitation is not pending")
	ErrInvitationExpired    = errors.New("invitation expired")
	ErrVersionConflict      = errors.New("version conflict")
)

// Invitation tracks an instructor invitation.
type Invitation struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	InvitedBy string    `json:"invitedBy"`
	Status    string    `json:"status"` // "pending", "accepted", "cancelled", "expired"
	TokenHash string    `json:"-"`
	Version   int       `json:"version"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store defines persistence for instructor invitations.
type Store interface {
	CreateInvitation(ctx context.Context, email, invitedBy string, ttl time.Duration) (*Invitation, string, error)
	GetInvitation(ctx context.Context, id string) (*Invitation, error)
	GetInvitationByToken(ctx context.Context, rawToken string) (*Invitation, error)
	AcceptInvitation(ctx context.Context, rawToken string) (*Invitation, error)
	CancelInvitation(ctx context.Context, id string, expectedVersion int) (*Invitation, error)
}

// MemoryStore provides in-memory invitation tracking.
type MemoryStore struct {
	mu          sync.RWMutex
	invitations map[string]*Invitation // id -> Invitation
	byToken     map[string]string      // tokenHash -> id
}

// NewMemoryStore initializes a new invitation store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		invitations: make(map[string]*Invitation),
		byToken:     make(map[string]string),
	}
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func generateRandomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateInvitation creates a new invitation with a random token.
func (s *MemoryStore) CreateInvitation(ctx context.Context, email, invitedBy string, ttl time.Duration) (*Invitation, string, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	s.mu.Lock()
	defer s.mu.Unlock()

	rawToken := generateRandomToken(16)
	tHash := hashToken(rawToken)
	now := time.Now()
	inv := &Invitation{
		ID:        fmt.Sprintf("inv_%s", generateRandomToken(8)),
		Email:     normEmail,
		InvitedBy: invitedBy,
		Status:    "pending",
		TokenHash: tHash,
		Version:   1,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.invitations[inv.ID] = inv
	s.byToken[tHash] = inv.ID

	invCopy := *inv
	return &invCopy, rawToken, nil
}

// GetInvitation looks up an invitation by ID.
func (s *MemoryStore) GetInvitation(ctx context.Context, id string) (*Invitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inv, exists := s.invitations[id]
	if !exists {
		return nil, ErrInvitationNotFound
	}
	invCopy := *inv
	return &invCopy, nil
}

// GetInvitationByToken looks up an invitation by token.
func (s *MemoryStore) GetInvitationByToken(ctx context.Context, rawToken string) (*Invitation, error) {
	tHash := hashToken(rawToken)
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.byToken[tHash]
	if !exists {
		return nil, ErrInvitationNotFound
	}
	invCopy := *s.invitations[id]
	return &invCopy, nil
}

// AcceptInvitation marks the invitation accepted.
func (s *MemoryStore) AcceptInvitation(ctx context.Context, rawToken string) (*Invitation, error) {
	tHash := hashToken(rawToken)
	s.mu.Lock()
	defer s.mu.Unlock()

	id, exists := s.byToken[tHash]
	if !exists {
		return nil, ErrInvitationNotFound
	}

	inv := s.invitations[id]
	if inv.Status != "pending" {
		return nil, ErrInvitationNotPending
	}
	if time.Now().After(inv.ExpiresAt) {
		inv.Status = "expired"
		return nil, ErrInvitationExpired
	}

	inv.Status = "accepted"
	inv.Version++
	inv.UpdatedAt = time.Now()

	invCopy := *inv
	return &invCopy, nil
}

// CancelInvitation marks a pending invitation cancelled with optimistic version check.
func (s *MemoryStore) CancelInvitation(ctx context.Context, id string, expectedVersion int) (*Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inv, exists := s.invitations[id]
	if !exists {
		return nil, ErrInvitationNotFound
	}
	if inv.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	if inv.Status != "pending" {
		return nil, ErrInvitationNotPending
	}

	inv.Status = "cancelled"
	inv.Version++
	inv.UpdatedAt = time.Now()

	invCopy := *inv
	return &invCopy, nil
}
