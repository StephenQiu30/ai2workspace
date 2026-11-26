package notification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	notificationdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/notification"
)

const InaccessibleSummaryPlaceholder = "相关内容已不可访问"

// TargetAccessChecker checks whether a notification target is still accessible to userID.
type TargetAccessChecker func(ctx context.Context, userID, targetType, targetID string) bool

// Store defines persistence operations for notifications.
type Store interface {
	AppendNotification(ctx context.Context, eventID, recipientID, notifType, title, summary, targetType, targetID string) (*notificationdomain.Notification, error)
	ListNotifications(ctx context.Context, recipientID string, page, pageSize int) ([]*notificationdomain.Notification, int, error)
	MarkRead(ctx context.Context, recipientID, notificationID string) error
}

// MemoryStore provides a thread-safe in-memory store for notifications.
type MemoryStore struct {
	mu            sync.RWMutex
	notifications map[string]*notificationdomain.Notification // notifID -> Notification
	dedupIndex    map[string]string                           // "eventID:recipientID" -> notifID
	lastTimestamp time.Time
}

// NewMemoryStore initializes a new MemoryStore for notifications.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		notifications: make(map[string]*notificationdomain.Notification),
		dedupIndex:    make(map[string]string),
	}
}

func NewID(prefix string) string {
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

func cloneNotification(n *notificationdomain.Notification) *notificationdomain.Notification {
	if n == nil {
		return nil
	}
	cp := *n
	return &cp
}

// AppendNotification inserts a notification with deduplication on (eventID, recipientID).
func (s *MemoryStore) AppendNotification(ctx context.Context, eventID, recipientID, notifType, title, summary, targetType, targetID string) (*notificationdomain.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dedupKey := fmt.Sprintf("%s:%s", eventID, recipientID)
	if existingID, ok := s.dedupIndex[dedupKey]; ok {
		return cloneNotification(s.notifications[existingID]), nil
	}

	notifID := NewID("notif")
	notif := &notificationdomain.Notification{
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
func (s *MemoryStore) ListNotifications(ctx context.Context, recipientID string, page, pageSize int) ([]*notificationdomain.Notification, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var userList []*notificationdomain.Notification
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
		return []*notificationdomain.Notification{}, total, nil
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
		return notificationdomain.ErrNotificationNotFound
	}
	if n.RecipientID != recipientID {
		return notificationdomain.ErrPermissionDenied
	}

	n.Read = true
	return nil
}
