package notification

import (
	"errors"
	"time"
)

var (
	// ErrNotificationNotFound indicates the requested notification does not exist.
	ErrNotificationNotFound = errors.New("notification not found")
	// ErrPermissionDenied indicates the caller is not the notification recipient.
	ErrPermissionDenied = errors.New("permission denied: recipient mismatch")
)

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
