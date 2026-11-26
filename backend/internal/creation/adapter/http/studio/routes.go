package studio

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes adds studio endpoints to the application router.
func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/studio/workspaces/:workspaceId/access", h.access)
}
