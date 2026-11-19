package notification

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// AppendNotificationRequest defines internal payload from order/learning events.
type AppendNotificationRequest struct {
	EventID     string `json:"eventId" binding:"required"`
	RecipientID string `json:"recipientId" binding:"required"`
	Type        string `json:"type" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Summary     string `json:"summary"`
}

// NotificationListResponse represents paginated notifications list.
type NotificationListResponse struct {
	Items []*Notification `json:"items"`
	Total int             `json:"total"`
	Page  int             `json:"page"`
	Limit int             `json:"limit"`
}

// Handler manages notification HTTP endpoints.
type Handler struct {
	store Store
}

// NewHandler creates a notification Handler.
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// AppendNotification handles internal event delivery with strict deduplication.
// @ID communityAppendNotification
// @Summary Append notification from domain event (Internal)
// @Tags Community
// @Accept json
// @Produce json
// @Param request body AppendNotificationRequest true "Notification payload"
// @Success 201 {object} Notification
// @Failure 400 {object} map[string]string
// @Router /internal/community/notifications [post]
func (h *Handler) AppendNotification(c *gin.Context) {
	var req AppendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	notif, err := h.store.AppendNotification(c.Request.Context(), req.EventID, req.RecipientID, req.Type, req.Title, req.Summary)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, notif)
}

// ListMyNotifications lists notifications for authenticated user.
// @ID communityListMyNotifications
// @Summary List current user notifications
// @Tags Community
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} NotificationListResponse
// @Router /api/community/notifications [get]
func (h *Handler) ListMyNotifications(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "mock_recipient"
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	items, total, err := h.store.ListNotifications(c.Request.Context(), userID, page, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, NotificationListResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// MarkNotificationRead marks user notification as read.
// @ID communityMarkNotificationRead
// @Summary Mark notification as read
// @Tags Community
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/community/notifications/{id}/read [post]
func (h *Handler) MarkNotificationRead(c *gin.Context) {
	notifID := c.Param("id")
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "mock_recipient"
	}

	if err := h.store.MarkRead(c.Request.Context(), userID, notifID); err != nil {
		switch {
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrNotificationNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "marked_read"})
}
