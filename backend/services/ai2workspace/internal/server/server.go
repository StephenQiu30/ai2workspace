// Package server assembles the service's HTTP routes.
package server

import (
	"net/http"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/ai2workspace/internal/apidocs"
	"github.com/StephenQiu30/ai2workspace/backend/services/ai2workspace/internal/health"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// New returns an independent router. API documentation is opt-in.
func New(swaggerEnabled bool) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/api/ai2workspace/v1/health", health.Get)
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	})
	if swaggerEnabled {
		router.GET("/swagger/ai2workspace/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}
	return router
}
