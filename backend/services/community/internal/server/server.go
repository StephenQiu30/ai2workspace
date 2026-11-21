// Package server assembles HTTP routes for the community service.
package server

import (
	"net/http"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/community/api"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/health"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/interaction"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/moderation"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/notification"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/posts"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// New initializes an independent community router with default in-memory stores.
func New(swaggerEnabled bool) *gin.Engine {
	postStore := posts.NewMemoryStore(func(courseID, userID string) (bool, error) {
		// Mock course access verifier: allows non-empty userID
		return userID != "" && userID != "unauthorized_user", nil
	})
	interactStore := interaction.NewMemoryStore()
	modStore := moderation.NewMemoryStore(postStore)
	notifStore := notification.NewMemoryStore()

	return NewWithStores(swaggerEnabled, postStore, interactStore, modStore, notifStore)
}

// NewWithStores returns a router configured with specific stores.
func NewWithStores(
	swaggerEnabled bool,
	postStore posts.Store,
	interactStore interaction.Store,
	modStore moderation.Store,
	notifStore notification.Store,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	postsHandler := posts.NewHandler(postStore)
	interactHandler := interaction.NewHandler(interactStore)
	modHandler := moderation.NewHandler(modStore)
	notifHandler := notification.NewHandler(notifStore)

	// Health
	router.GET("/api/community/health", health.Get)

	// Posts & Comments
	router.POST("/api/community/posts", postsHandler.CreatePost)
	router.GET("/api/community/posts/:id", postsHandler.GetPost)
	router.PATCH("/api/community/posts/:id", postsHandler.UpdatePost)
	router.DELETE("/api/community/posts/:id", postsHandler.DeletePost)
	router.GET("/api/community/posts", postsHandler.ListPosts)
	router.POST("/api/community/posts/:id/comments", postsHandler.CreateComment)
	router.DELETE("/api/community/comments/:id", postsHandler.DeleteComment)

	// Reactions & Favorites
	router.POST("/api/community/reactions", interactHandler.SetReaction)
	router.GET("/api/community/reactions", interactHandler.GetReaction)
	router.POST("/api/community/favorites", interactHandler.SetFavorite)
	router.GET("/api/community/me/favorites", interactHandler.ListMyFavorites)

	// Moderation Reports
	router.POST("/api/community/reports", modHandler.CreateReport)
	router.GET("/api/community/reports", modHandler.ListReports)
	router.POST("/api/community/reports/:id/review", modHandler.ModerateReport)

	// Notifications
	router.POST("/internal/community/notifications", notifHandler.AppendNotification)
	router.GET("/api/community/notifications", notifHandler.ListMyNotifications)
	router.POST("/api/community/notifications/:id/read", notifHandler.MarkNotificationRead)

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	})

	if swaggerEnabled {
		router.GET("/swagger/community/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	return router
}
