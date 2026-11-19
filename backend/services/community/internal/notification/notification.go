// Package notification manages user in-app notifications, delivery idempotency, and isolation.
package notification

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
	ErrNotificationNotFound = errors.New("notification not found")
	ErrPermissionDenied     = errors.New("permission denied: recipient mismatch")
)

// Notification represents a user event notification.
type Notification struct {
	ID          string    `json:"id"`
	EventID     string    `json:"eventId"`
	RecipientID string    `json:"recipientId"`
	Type        string    `json:"type"` // "order", "refund", "reply", "moderation", "system"
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Read        bool      `json:"read"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Store defines persistence operations for notifications.
type Store interface {
	AppendNotification(ctx context.Context, eventID, recipientID, notifType, title, summary string) (*Notification, error)
	ListNotifications(ctx context.Context, recipientID string, page, limit int) ([]*Notification, int, error)
	MarkRead(ctx context.Context, recipientID, notificationID string) error
}

// MemoryStore provides thread-safe in-memory store for notifications.
type MemoryStore struct {
	mu            sync.RWMutex
	notifications map[string]*Notification // notifID -> Notification
	dedupIndex    map[string]string        // "eventID:recipientID" -> notifID
}

// NewMemoryStore initializes a new MemoryStore for notifications.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		notifications: make(map[string]*Notification),
		dedupIndex:    make(map[string]string),
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (s *MemoryStore) AppendNotification(ctx context.Context, eventID, recipientID, notifType, title, summary string) (*Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Dedup check on (eventID, recipientID)
	dedupKey := fmt.Sprintf("%s:%s", eventID, recipientID)
	if existingID, ok := s.dedupIndex[dedupKey]; ok {
		return s.notifications[existingID], nil
	}

	notifID := generateID("notif")
	notif := &Notification{
		ID:          notifID,
		EventID:     eventID,
		RecipientID: recipientID,
		Type:        notifType,
		Title:       title,
		Summary:     summary,
		Read:        false,
		CreatedAt:   time.Now(),
	}

	s.notifications[notifID] = notif
	s.dedupIndex[dedupKey] = notifID
	return notif, nil
}

func (s *MemoryStore) ListNotifications(ctx context.Context, recipientID string, page, limit int) ([]*Notification, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var userList []*Notification
	for _, n := range s.notifications {
		if n.RecipientID == recipientID {
			userList = append(userList, n)
		}
	}

	total := len(userList)
	start := (page - 1) * limit
	if start >= total {
		return []*Notification{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	return userList[start:end], total, nil
}

func (s *MemoryStore) MarkRead(ctx context.Context, recipientID, notificationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.notifications[notificationID]
	if !ok {
		return ErrNotificationNotFound
	}
	if n.RecipientID != recipientID {
		return ErrPermissionDenied
	}

	n.Read = true
	return nil
}
