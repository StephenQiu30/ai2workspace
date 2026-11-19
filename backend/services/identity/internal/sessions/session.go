// Package sessions manages opaque session tokens and user login state.
package sessions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
	ErrSessionRevoked  = errors.New("session revoked")
)

const (
	SessionCookieName = "ai2_session"
	DefaultSessionTTL = 7 * 24 * time.Hour
)

// Session represents an active or revoked user login session.
type Session struct {
	ID        string    `json:"id"` // token hash
	UserID    string    `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
	Revoked   bool      `json:"revoked"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
}

// Store defines session lifecycle persistence.
type Store interface {
	CreateSession(ctx context.Context, userID string, ttl time.Duration) (string, *Session, error)
	GetSession(ctx context.Context, rawToken string) (*Session, error)
	GetUserIDBySession(ctx context.Context, rawToken string) (string, error)
	RevokeSession(ctx context.Context, rawToken string) error
	RevokeAllForUser(ctx context.Context, userID string) error
}

// MemoryStore provides thread-safe in-memory session tracking.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session // sessionID (hash) -> Session
	byUser   map[string][]string // userID -> []sessionID
}

// NewMemoryStore initializes an in-memory session store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*Session),
		byUser:   make(map[string][]string),
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

// CreateSession creates a random session token, hashes it for storage, and returns the raw token.
func (s *MemoryStore) CreateSession(ctx context.Context, userID string, ttl time.Duration) (string, *Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rawToken := generateRandomToken(32)
	sID := hashToken(rawToken)
	now := time.Now()

	sess := &Session{
		ID:        sID,
		UserID:    userID,
		ExpiresAt: now.Add(ttl),
		Revoked:   false,
		Version:   1,
		CreatedAt: now,
	}

	s.sessions[sID] = sess
	s.byUser[userID] = append(s.byUser[userID], sID)

	sessCopy := *sess
	return rawToken, &sessCopy, nil
}

// GetSession returns the active session, checking expiration and revocation.
func (s *MemoryStore) GetSession(ctx context.Context, rawToken string) (*Session, error) {
	sID := hashToken(rawToken)
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, exists := s.sessions[sID]
	if !exists {
		return nil, ErrSessionNotFound
	}
	if sess.Revoked {
		return nil, ErrSessionRevoked
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	sessCopy := *sess
	return &sessCopy, nil
}

// GetUserIDBySession returns the user ID for a valid session.
func (s *MemoryStore) GetUserIDBySession(ctx context.Context, rawToken string) (string, error) {
	sess, err := s.GetSession(ctx, rawToken)
	if err != nil {
		return "", err
	}
	return sess.UserID, nil
}

// RevokeSession revokes a single session by raw token.
func (s *MemoryStore) RevokeSession(ctx context.Context, rawToken string) error {
	sID := hashToken(rawToken)
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, exists := s.sessions[sID]
	if !exists {
		return nil // idempotent
	}
	sess.Revoked = true
	sess.Version++
	return nil
}

// RevokeAllForUser revokes all sessions associated with a specific user.
func (s *MemoryStore) RevokeAllForUser(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sID := range s.byUser[userID] {
		if sess, exists := s.sessions[sID]; exists {
			sess.Revoked = true
			sess.Version++
		}
	}
	return nil
}
