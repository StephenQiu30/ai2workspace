package access

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	accessdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/access"
)

// Store defines persistence for instructor invitations and audit logs.
type Store interface {
	CreateInvitation(ctx context.Context, email, invitedBy string, ttl time.Duration) (*accessdomain.Invitation, string, error)
	GetInvitation(ctx context.Context, id string) (*accessdomain.Invitation, error)
	GetInvitationByToken(ctx context.Context, rawToken string) (*accessdomain.Invitation, error)
	ListInvitations(ctx context.Context, filter accessdomain.ListInvitationsFilter) ([]*accessdomain.Invitation, int, error)
	AcceptInvitation(ctx context.Context, rawToken string) (*accessdomain.Invitation, error)
	CancelInvitation(ctx context.Context, id string, expectedVersion int) (*accessdomain.Invitation, error)
	RecordAuditLog(ctx context.Context, entry accessdomain.AuditLog) error
	ListAuditLogs(ctx context.Context, targetID string) ([]accessdomain.AuditLog, error)
}

// MemoryStore provides thread-safe in-memory invitation and audit log storage.
type MemoryStore struct {
	mu          sync.RWMutex
	invitations map[string]*accessdomain.Invitation // id -> Invitation
	byToken     map[string]string                   // tokenHash -> id
	auditLogs   []accessdomain.AuditLog
}

// NewMemoryStore initializes a new invitation and audit store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		invitations: make(map[string]*accessdomain.Invitation),
		byToken:     make(map[string]string),
		auditLogs:   make([]accessdomain.AuditLog, 0),
	}
}

func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func GenerateRandomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func NewID(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, GenerateRandomToken(8))
}

func NormalizePagination(page, pageSize int) (int, int) {
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
func (s *MemoryStore) CreateInvitation(ctx context.Context, email, invitedBy string, ttl time.Duration) (*accessdomain.Invitation, string, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	s.mu.Lock()
	defer s.mu.Unlock()

	rawToken := GenerateRandomToken(16)
	tHash := HashToken(rawToken)
	now := time.Now()
	inv := &accessdomain.Invitation{
		ID:        NewID("inv"),
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
func (s *MemoryStore) GetInvitation(ctx context.Context, id string) (*accessdomain.Invitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inv, exists := s.invitations[id]
	if !exists {
		return nil, accessdomain.ErrInvitationNotFound
	}
	invCopy := *inv
	return &invCopy, nil
}

// GetInvitationByToken looks up an invitation by token.
func (s *MemoryStore) GetInvitationByToken(ctx context.Context, rawToken string) (*accessdomain.Invitation, error) {
	tHash := HashToken(rawToken)
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.byToken[tHash]
	if !exists {
		return nil, accessdomain.ErrInvitationNotFound
	}
	invCopy := *s.invitations[id]
	return &invCopy, nil
}

// ListInvitations returns invitations ordered by createdAt DESC with optional status filtering.
func (s *MemoryStore) ListInvitations(ctx context.Context, filter accessdomain.ListInvitationsFilter) ([]*accessdomain.Invitation, int, error) {
	page, pageSize := NormalizePagination(filter.Page, filter.PageSize)
	statusFilter := strings.TrimSpace(filter.Status)
	now := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()

	matched := make([]*accessdomain.Invitation, 0, len(s.invitations))
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
		return []*accessdomain.Invitation{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return matched[start:end], total, nil
}

// AcceptInvitation marks the invitation accepted.
func (s *MemoryStore) AcceptInvitation(ctx context.Context, rawToken string) (*accessdomain.Invitation, error) {
	tHash := HashToken(rawToken)
	s.mu.Lock()
	defer s.mu.Unlock()

	id, exists := s.byToken[tHash]
	if !exists {
		return nil, accessdomain.ErrInvitationNotFound
	}

	inv := s.invitations[id]
	if inv.Status != "pending" {
		return nil, accessdomain.ErrInvitationNotPending
	}
	if time.Now().After(inv.ExpiresAt) {
		inv.Status = "expired"
		return nil, accessdomain.ErrInvitationExpired
	}

	inv.Status = "accepted"
	inv.Version++
	inv.UpdatedAt = time.Now()

	invCopy := *inv
	return &invCopy, nil
}

// CancelInvitation marks a pending invitation cancelled with optimistic version check.
func (s *MemoryStore) CancelInvitation(ctx context.Context, id string, expectedVersion int) (*accessdomain.Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inv, exists := s.invitations[id]
	if !exists {
		return nil, accessdomain.ErrInvitationNotFound
	}
	if inv.Version != expectedVersion {
		return nil, accessdomain.ErrVersionConflict
	}
	if inv.Status != "pending" {
		return nil, accessdomain.ErrInvitationNotPending
	}

	inv.Status = "cancelled"
	inv.Version++
	inv.UpdatedAt = time.Now()

	invCopy := *inv
	return &invCopy, nil
}

// RecordAuditLog appends an audit entry.
func (s *MemoryStore) RecordAuditLog(ctx context.Context, entry accessdomain.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.ID == "" {
		entry.ID = NewID("aud")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	s.auditLogs = append(s.auditLogs, entry)
	return nil
}

// ListAuditLogs returns audit entries filtered by targetID (or all if empty), newest first.
func (s *MemoryStore) ListAuditLogs(ctx context.Context, targetID string) ([]accessdomain.AuditLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]accessdomain.AuditLog, 0)
	for i := len(s.auditLogs) - 1; i >= 0; i-- {
		entry := s.auditLogs[i]
		if targetID == "" || entry.TargetID == targetID {
			result = append(result, entry)
		}
	}
	return result, nil
}
