package notification

import (
	"errors"
	"net/http"
	"strconv"

	authhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/auth"
	notificationapp "github.com/StephenQiu30/ai2workspace/backend/internal/community/application/notification"
	notificationdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/notification"
	"github.com/gin-gonic/gin"
)

// Handler manages notification HTTP endpoints.
type Handler struct {
	store    notificationapp.Store
	resolver authhttp.SessionResolver

	targetChecker notificationapp.TargetAccessChecker
}

// NewHandler creates a notification Handler with service-token verification, session auth, and target accessibility checks.
func NewHandler(
	store notificationapp.Store,
	resolver authhttp.SessionResolver,

	targetChecker notificationapp.TargetAccessChecker,
) *Handler {
	return &Handler{
		store:    store,
		resolver: resolver,

		targetChecker: targetChecker,
	}
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
// @Failure 401 {object} authhttp.ErrorResponse
// @Failure 403 {object} authhttp.ErrorResponse
// @Router /api/community/notifications [get]
func (h *Handler) ListMyNotifications(c *gin.Context) {
	actor, ok := authhttp.RequireAuthenticated(c, h.resolver)
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
		c.JSON(http.StatusInternalServerError, authhttp.ErrorResponse{Error: "list_failed", Message: err.Error()})
		return
	}

	if h.targetChecker != nil {
		for _, item := range items {
			if (item.TargetType == "post" || item.TargetType == "comment") && item.TargetID != "" {
				if !h.targetChecker(c.Request.Context(), actor.UserID, item.TargetType, item.TargetID) {
					item.TargetAccessible = false
					item.Summary = notificationapp.InaccessibleSummaryPlaceholder
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
// @Failure 401 {object} authhttp.ErrorResponse
// @Failure 403 {object} authhttp.ErrorResponse
// @Failure 404 {object} authhttp.ErrorResponse
// @Router /api/community/notifications/{id}/read [post]
func (h *Handler) MarkNotificationRead(c *gin.Context) {
	actor, ok := authhttp.RequireAuthenticated(c, h.resolver)
	if !ok {
		return
	}

	notifID := c.Param("id")
	if err := h.store.MarkRead(c.Request.Context(), actor.UserID, notifID); err != nil {
		switch {
		case errors.Is(err, notificationdomain.ErrPermissionDenied):
			c.JSON(http.StatusForbidden, authhttp.ErrorResponse{Error: "forbidden", Message: err.Error()})
		case errors.Is(err, notificationdomain.ErrNotificationNotFound):
			c.JSON(http.StatusNotFound, authhttp.ErrorResponse{Error: "not_found", Message: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, authhttp.ErrorResponse{Error: "mark_read_failed", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, MarkReadResponse{Status: "marked_read"})
}
