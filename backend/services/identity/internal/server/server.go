// Package server assembles the service's HTTP routes.
package server

import (
	"net/http"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/identity/api"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/health"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// New returns an independent router. API documentation is opt-in.
func New(swaggerEnabled bool) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/api/identity/v1/health", health.Get)
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	})
	if swaggerEnabled {
		router.GET("/swagger/identity/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}
	return router
}
