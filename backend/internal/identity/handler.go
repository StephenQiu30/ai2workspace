package identity

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const workspaceMembers = "identity.workspace_members"

// Handler reads workspace membership from the identity schema.
type Handler struct {
	db      *gorm.DB
	session *SessionClient
}

// NewHandler connects the workspace endpoints to the database and session authority.
func NewHandler(db *gorm.DB, session *SessionClient) *Handler {
	return &Handler{db: db, session: session}
}

// HealthResponse describes whether the workspace endpoints can query their schema.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// AccessResponse identifies the caller's active workspace role.
type AccessResponse struct {
	WorkspaceID string `json:"workspaceId" example:"workshop"`
	Role        string `json:"role" enums:"owner,member" example:"member"`
}

// ErrorResponse is a stable error code for API consumers.
type ErrorResponse struct {
	Error string `json:"error" example:"workspace_access_denied"`
}

// Health checks PostgreSQL and the migration-owned workspace membership table.
// @Summary Check studio readiness
// @Tags studio
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} HealthResponse
// @Router /api/studio/health [get]
// @ID studioGetHealth
func (h *Handler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.WithContext(ctx).Exec("SELECT role FROM " + workspaceMembers + " LIMIT 0").Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, HealthResponse{Status: "unavailable"})
		return
	}
	c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}

// Access returns the authenticated user's role for the requested workspace.
// @Summary Check current user's workspace access
// @Tags studio
// @Produce json
// @Param workspaceId path string true "Workspace ID"
// @Success 200 {object} AccessResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/studio/workspaces/{workspaceId}/access [get]
// @ID studioGetWorkspaceAccess
func (h *Handler) Access(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	cookie, err := c.Request.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "authentication_required"})
		return
	}
	userID, status := h.session.userID(c.Request.Context(), cookie)
	switch status {
	case http.StatusUnauthorized:
		c.JSON(status, ErrorResponse{Error: "authentication_required"})
		return
	case http.StatusForbidden:
		c.JSON(status, ErrorResponse{Error: "workspace_access_denied"})
		return
	case http.StatusServiceUnavailable:
		c.JSON(status, ErrorResponse{Error: "identity_unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	workspaceID := c.Param("workspaceId")
	var member struct{ Role string }
	err = h.db.WithContext(ctx).Table(workspaceMembers).Select("role").
		Where("workspace_id = ? AND user_id = ? AND revoked_at IS NULL", workspaceID, userID).
		Take(&member).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "workspace_access_denied"})
	case err != nil:
		slog.ErrorContext(ctx, "query workspace membership", "error", err)
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "workspace_access_unavailable"})
	default:
		c.JSON(http.StatusOK, AccessResponse{WorkspaceID: workspaceID, Role: member.Role})
	}
}
