package notification

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/auth"
	"github.com/gin-gonic/gin"
)

// AppendNotificationRequest defines the internal payload delivered from domain events.
type AppendNotificationRequest struct {
	EventID     string `json:"eventId" binding:"required" example:"evt_order_123"`
	RecipientID string `json:"recipientId" binding:"required" example:"usr_12345"`
	Type        string `json:"type" binding:"required" example:"order"`
	Title       string `json:"title" binding:"required" example:"课程购买成功"`
	Summary     string `json:"summary" example:"您已获得课程访问权限"`
	TargetType  string `json:"targetType,omitempty" example:"post"`
	TargetID    string `json:"targetId,omitempty" example:"post_12345"`
}

// NotificationListResponse represents a paginated list of user notifications.
type NotificationListResponse struct {
	Items    []*Notification `json:"items"`
	Total    int             `json:"total" example:"5"`
	Page     int             `json:"page" example:"1"`
	PageSize int             `json:"pageSize" example:"20"`
}

// MarkReadResponse represents the acknowledgement for marking a notification as read.
type MarkReadResponse struct {
	Status string `json:"status" example:"marked_read"`
}

// Handler manages notification HTTP endpoints.
type Handler struct {
	store         Store
	resolver      auth.SessionResolver
	serviceToken  string
	targetChecker TargetAccessChecker
}

// NewHandler creates a notification Handler with service-token verification, session auth, and target accessibility checks.
func NewHandler(
	store Store,
	resolver auth.SessionResolver,
	serviceToken string,
	targetChecker TargetAccessChecker,
) *Handler {
	return &Handler{
		store:         store,
		resolver:      resolver,
		serviceToken:  serviceToken,
		targetChecker: targetChecker,
	}
}

// AppendNotification handles internal event delivery with strict X-Service-Token verification and deduplication.
// @ID communityAppendNotification
// @Summary Append notification from domain event (Internal)
// @Tags Community
// @Accept json
// @Produce json
// @Param X-Service-Token header string true "Service authentication token"
// @Param request body AppendNotificationRequest true "Notification payload"
// @Success 201 {object} Notification
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Router /internal/community/notifications [post]
func (h *Handler) AppendNotification(c *gin.Context) {
	if !auth.VerifyServiceToken(c, h.serviceToken) {
		c.JSON(http.StatusUnauthorized, auth.ErrorResponse{
			Error:   "unauthorized_service",
			Message: "valid service token required",
		})
		return
	}

	var req AppendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	notif, err := h.store.AppendNotification(
		c.Request.Context(),
		strings.TrimSpace(req.EventID),
		strings.TrimSpace(req.RecipientID),
		strings.TrimSpace(req.Type),
		strings.TrimSpace(req.Title),
		strings.TrimSpace(req.Summary),
		strings.TrimSpace(req.TargetType),
		strings.TrimSpace(req.TargetID),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "append_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, notif)
}

// ListMyNotifications lists the current authenticated user's notifications in reverse chronological order.
// Notifications targeting hidden, deleted, or inaccessible content are marked targetAccessible=false with sanitized summary.
// @ID communityListMyNotifications
// @Summary List current user notifications
// @Tags Community
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param pageSize query int false "Page size (default 20, max 100)"
// @Success 200 {object} NotificationListResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Router /api/community/notifications [get]
func (h *Handler) ListMyNotifications(c *gin.Context) {
	actor, ok := auth.RequireAuthenticated(c, h.resolver)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	sizeParam := c.Query("pageSize")
	if sizeParam == "" {
		sizeParam = c.DefaultQuery("limit", "20")
	}
	pageSize, _ := strconv.Atoi(sizeParam)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	items, total, err := h.store.ListNotifications(c.Request.Context(), actor.UserID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "list_failed", Message: err.Error()})
		return
	}

	if h.targetChecker != nil {
		for _, item := range items {
			if (item.TargetType == "post" || item.TargetType == "comment") && item.TargetID != "" {
				if !h.targetChecker(c.Request.Context(), actor.UserID, item.TargetType, item.TargetID) {
					item.TargetAccessible = false
					item.Summary = inaccessibleSummaryPlaceholder
				}
			}
		}
	}

	c.JSON(http.StatusOK, NotificationListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// MarkNotificationRead marks one of the current user's notifications as read.
// @ID communityMarkNotificationRead
// @Summary Mark notification as read
// @Tags Community
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} MarkReadResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Router /api/community/notifications/{id}/read [post]
func (h *Handler) MarkNotificationRead(c *gin.Context) {
	actor, ok := auth.RequireAuthenticated(c, h.resolver)
	if !ok {
		return
	}

	notifID := c.Param("id")
	if err := h.store.MarkRead(c.Request.Context(), actor.UserID, notifID); err != nil {
		switch {
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "forbidden", Message: err.Error()})
		case errors.Is(err, ErrNotificationNotFound):
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "not_found", Message: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "mark_read_failed", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, MarkReadResponse{Status: "marked_read"})
}
