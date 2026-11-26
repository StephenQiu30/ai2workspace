package studio

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/internal/creation/application"
	"github.com/StephenQiu30/ai2workspace/backend/internal/creation/domain"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/gin-gonic/gin"
)

// Handler serves workspace access requests.
type Handler struct {
	workspace *application.Access
}

// NewHandler builds the workspace access HTTP handler.
func NewHandler(workspace *application.Access) *Handler {
	return &Handler{workspace: workspace}
}

// access returns the authenticated user's role for the requested workspace.
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
func (h *Handler) access(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	token, err := c.Cookie(identity.SessionCookieName)
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "authentication_required"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	workspaceID := c.Param("workspaceId")
	role, err := h.workspace.Role(ctx, token, workspaceID)
	switch {
	case errors.Is(err, identity.ErrInvalidSession):
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "authentication_required"})
	case errors.Is(err, identity.ErrInactiveAccount), errors.Is(err, domain.ErrMembershipNotFound):
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "workspace_access_denied"})
	case errors.Is(err, application.ErrIdentityUnavailable):
		slog.ErrorContext(ctx, "resolve session", "error", err)
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "identity_unavailable"})
	case err != nil:
		slog.ErrorContext(ctx, "query workspace membership", "error", err)
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "workspace_access_unavailable"})
	default:
		c.JSON(http.StatusOK, AccessResponse{WorkspaceID: workspaceID, Role: string(role)})
	}
}
