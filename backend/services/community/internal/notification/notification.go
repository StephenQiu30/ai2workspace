// Package notification manages user in-app notifications, delivery deduplication, and recipient isolation.
package notification

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
	// ErrNotificationNotFound indicates the requested notification does not exist.
	ErrNotificationNotFound = errors.New("notification not found")
	// ErrPermissionDenied indicates the caller is not the notification recipient.
	ErrPermissionDenied = errors.New("permission denied: recipient mismatch")
)

const inaccessibleSummaryPlaceholder = "相关内容已不可访问"

// TargetAccessChecker checks whether a notification target is still accessible to userID.
type TargetAccessChecker func(ctx context.Context, userID, targetType, targetID string) bool

// Notification represents a user in-app event notification.
type Notification struct {
	ID               string    `json:"id" example:"notif_12345"`
	EventID          string    `json:"eventId" example:"comment_reply:cmt_12345"`
	RecipientID      string    `json:"recipientId" example:"usr_12345"`
	Type             string    `json:"type" example:"reply"` // "order", "refund", "reply", "moderation", "system"
	Title            string    `json:"title" example:"收到新的社区回复"`
	Summary          string    `json:"summary" example:"小屿学员 回复了讨论《学习笔记》"`
	TargetType       string    `json:"targetType,omitempty" example:"post"`
	TargetID         string    `json:"targetId,omitempty" example:"post_12345"`
	TargetAccessible bool      `json:"targetAccessible" example:"true"`
	Read             bool      `json:"read" example:"false"`
	CreatedAt        time.Time `json:"createdAt"`
}

// Store defines persistence operations for notifications.
type Store interface {
	AppendNotification(ctx context.Context, eventID, recipientID, notifType, title, summary, targetType, targetID string) (*Notification, error)
	ListNotifications(ctx context.Context, recipientID string, page, pageSize int) ([]*Notification, int, error)
	MarkRead(ctx context.Context, recipientID, notificationID string) error
}

// MemoryStore provides a thread-safe in-memory store for notifications.
type MemoryStore struct {
	mu            sync.RWMutex
	notifications map[string]*Notification // notifID -> Notification
	dedupIndex    map[string]string        // "eventID:recipientID" -> notifID
	lastTimestamp time.Time
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

func (s *MemoryStore) nextTimestampLocked() time.Time {
	now := time.Now().UTC()
	if !now.After(s.lastTimestamp) {
		now = s.lastTimestamp.Add(time.Microsecond)
	}
	s.lastTimestamp = now
	return now
}

func cloneNotification(n *Notification) *Notification {
	if n == nil {
		return nil
	}
	cp := *n
	return &cp
}

// AppendNotification inserts a notification with deduplication on (eventID, recipientID).
func (s *MemoryStore) AppendNotification(ctx context.Context, eventID, recipientID, notifType, title, summary, targetType, targetID string) (*Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dedupKey := fmt.Sprintf("%s:%s", eventID, recipientID)
	if existingID, ok := s.dedupIndex[dedupKey]; ok {
		return cloneNotification(s.notifications[existingID]), nil
	}

	notifID := generateID("notif")
	notif := &Notification{
		ID:               notifID,
		EventID:          eventID,
		RecipientID:      recipientID,
		Type:             notifType,
		Title:            title,
		Summary:          summary,
		TargetType:       strings.TrimSpace(targetType),
		TargetID:         strings.TrimSpace(targetID),
		TargetAccessible: true,
		Read:             false,
		CreatedAt:        s.nextTimestampLocked(),
	}

	s.notifications[notifID] = notif
	s.dedupIndex[dedupKey] = notifID
	return cloneNotification(notif), nil
}

// ListNotifications returns recipient notifications ordered deterministically by CreatedAt DESC, ID DESC.
func (s *MemoryStore) ListNotifications(ctx context.Context, recipientID string, page, pageSize int) ([]*Notification, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var userList []*Notification
	for _, n := range s.notifications {
		if n.RecipientID == recipientID {
			cp := cloneNotification(n)
			cp.TargetAccessible = true
			userList = append(userList, cp)
		}
	}

	sort.Slice(userList, func(i, j int) bool {
		if !userList[i].CreatedAt.Equal(userList[j].CreatedAt) {
			return userList[i].CreatedAt.After(userList[j].CreatedAt)
		}
		return userList[i].ID > userList[j].ID
	})

	total := len(userList)
	start := (page - 1) * pageSize
	if start >= total {
		return []*Notification{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return userList[start:end], total, nil
}

// MarkRead marks a recipient's notification as read.
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
