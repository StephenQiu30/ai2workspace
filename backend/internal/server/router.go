// Package server assembles the modular backend's HTTP routes.
package server

import (
	"net/http"

	_ "github.com/StephenQiu30/ai2workspace/backend/api"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// NewRouter wires the pilot endpoints and optionally serves the generated Swagger.
func NewRouter(workspaces *identity.Handler, swaggerEnabled bool) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/api/studio/health", workspaces.Health)
	router.GET("/api/studio/workspaces/:workspaceId/access", workspaces.Access)
	if swaggerEnabled {
		router.GET("/swagger/studio/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}
	return router
}
