package app

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// newRouter registers the endpoints shared by the application.
func newRouter(pool *pgxpool.Pool, swaggerEnabled bool) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/api/health", health(pool))
	router.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "not_found"}) })
	if swaggerEnabled {
		router.GET("/swagger/*path", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
	return router
}

// HealthResponse describes database connectivity.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
} // @name server.HealthResponse

// health checks the database shared by the application's modules.
// @Summary Check backend readiness
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} HealthResponse
// @Router /api/health [get]
// @ID getHealth
func health(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, HealthResponse{Status: "unavailable"})
			return
		}
		c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
	}
}
