// Package access manages admin authorizations, instructor invitations, audit logs, and internal service identity resolution.
package access

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
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

// ListInvitationsFilter defines optional filters and pagination for listing instructor invitations.
type ListInvitationsFilter struct {
	Status   string
	Page     int
	PageSize int
}

// AuditLog records administrative and sensitive role/status changes.
type AuditLog struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actorId"`
	TargetID  string    `json:"targetId"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Store defines persistence for instructor invitations and audit logs.
type Store interface {
	CreateInvitation(ctx context.Context, email, invitedBy string, ttl time.Duration) (*Invitation, string, error)
	GetInvitation(ctx context.Context, id string) (*Invitation, error)
	GetInvitationByToken(ctx context.Context, rawToken string) (*Invitation, error)
	ListInvitations(ctx context.Context, filter ListInvitationsFilter) ([]*Invitation, int, error)
	AcceptInvitation(ctx context.Context, rawToken string) (*Invitation, error)
	CancelInvitation(ctx context.Context, id string, expectedVersion int) (*Invitation, error)
	RecordAuditLog(ctx context.Context, entry AuditLog) error
	ListAuditLogs(ctx context.Context, targetID string) ([]AuditLog, error)
}

// MemoryStore provides thread-safe in-memory invitation and audit log storage.
type MemoryStore struct {
	mu          sync.RWMutex
	invitations map[string]*Invitation // id -> Invitation
	byToken     map[string]string      // tokenHash -> id
	auditLogs   []AuditLog
}

// NewMemoryStore initializes a new invitation and audit store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		invitations: make(map[string]*Invitation),
		byToken:     make(map[string]string),
		auditLogs:   make([]AuditLog, 0),
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

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, generateRandomToken(8))
}

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
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
		ID:        generateID("inv"),
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

// ListInvitations returns invitations ordered by createdAt DESC with optional status filtering.
func (s *MemoryStore) ListInvitations(ctx context.Context, filter ListInvitationsFilter) ([]*Invitation, int, error) {
	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	statusFilter := strings.TrimSpace(filter.Status)
	now := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()

	matched := make([]*Invitation, 0, len(s.invitations))
	for _, inv := range s.invitations {
		invCopy := *inv
		if invCopy.Status == "pending" && now.After(invCopy.ExpiresAt) {
			invCopy.Status = "expired"
		}
		if statusFilter != "" && invCopy.Status != statusFilter {
			continue
		}
		matched = append(matched, &invCopy)
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].ID > matched[j].ID
		}
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	total := len(matched)
	start := (page - 1) * pageSize
	if start >= total {
		return []*Invitation{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return matched[start:end], total, nil
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

// RecordAuditLog appends an audit entry.
func (s *MemoryStore) RecordAuditLog(ctx context.Context, entry AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.ID == "" {
		entry.ID = generateID("aud")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	s.auditLogs = append(s.auditLogs, entry)
	return nil
}

// ListAuditLogs returns audit entries filtered by targetID (or all if empty), newest first.
func (s *MemoryStore) ListAuditLogs(ctx context.Context, targetID string) ([]AuditLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]AuditLog, 0)
	for i := len(s.auditLogs) - 1; i >= 0; i-- {
		entry := s.auditLogs[i]
		if targetID == "" || entry.TargetID == targetID {
			result = append(result, entry)
		}
	}
	return result, nil
}
