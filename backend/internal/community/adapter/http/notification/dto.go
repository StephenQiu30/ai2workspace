package notification

import (
	notificationdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/notification"
)

// NotificationListResponse represents a paginated list of user notifications.
type NotificationListResponse struct {
	Items    []*notificationdomain.Notification `json:"items"`
	Total    int                                `json:"total" example:"5"`
	Page     int                                `json:"page" example:"1"`
	PageSize int                                `json:"pageSize" example:"20"`
}

// MarkReadResponse represents the acknowledgement for marking a notification as read.
type MarkReadResponse struct {
	Status string `json:"status" example:"marked_read"`
}
