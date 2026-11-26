// Package http binds endpoints for the community module.
package http

import (
	interactionhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/interaction"
	moderationhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/moderation"
	notificationhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/notification"
	postshttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/posts"
	"github.com/gin-gonic/gin"
)

// Routes holds the HTTP handlers for the community module.
type Routes struct {
	Posts        *postshttp.Handler
	Interaction  *interactionhttp.Handler
	Moderation   *moderationhttp.Handler
	Notification *notificationhttp.Handler
}

// RegisterRoutes adds community endpoints to the application router.
func (r Routes) RegisterRoutes(router gin.IRouter) {
	// Posts, Topics & Comments
	router.POST("/api/community/posts", r.Posts.CreatePost)
	router.GET("/api/community/posts", r.Posts.ListPosts)
	router.GET("/api/community/posts/:id", r.Posts.GetPost)
	router.PATCH("/api/community/posts/:id", r.Posts.UpdatePost)
	router.DELETE("/api/community/posts/:id", r.Posts.DeletePost)
	router.GET("/api/community/topics", r.Posts.ListTopics)
	router.POST("/api/community/posts/:id/comments", r.Posts.CreateComment)
	router.PATCH("/api/community/comments/:id", r.Posts.UpdateComment)
	router.DELETE("/api/community/comments/:id", r.Posts.DeleteComment)

	// Reactions & Favorites
	router.PUT("/api/community/reactions", r.Interaction.SetReaction)
	router.GET("/api/community/reactions", r.Interaction.GetReaction)
	router.PUT("/api/community/favorites", r.Interaction.SetFavorite)
	router.GET("/api/community/favorites/:postId", r.Interaction.GetFavoriteStatus)
	router.GET("/api/community/me/favorites", r.Interaction.ListMyFavorites)

	// Moderation Reports
	router.POST("/api/community/reports", r.Moderation.CreateReport)
	router.GET("/api/community/reports", r.Moderation.ListReports)
	router.POST("/api/community/reports/:id/review", r.Moderation.ModerateReport)

	// Notifications
	router.GET("/api/community/notifications", r.Notification.ListMyNotifications)
	router.POST("/api/community/notifications/:id/read", r.Notification.MarkNotificationRead)
}
