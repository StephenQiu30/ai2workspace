// Package server assembles HTTP routes and dependencies for the community service.
package server

import (
	"context"
	"net/http"
	"time"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/community/api"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/health"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/interaction"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/moderation"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/notification"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/posts"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Deps encapsulates injectable stores, authentication resolvers, and runtime flags.
type Deps struct {
	SwaggerEnabled      bool
	ServiceToken        string
	PostStore           posts.Store
	InteractionStore    interaction.Store
	ModerationStore     moderation.Store
	NotificationStore   notification.Store
	SessionResolver     auth.SessionResolver
	CourseAccessChecker auth.CourseAccessChecker
}

// New initializes a standalone community router with in-memory stores and fail-closed auth.
func New(swaggerEnabled bool) *gin.Engine {
	return NewWithDeps(Deps{
		SwaggerEnabled: swaggerEnabled,
		ServiceToken:   "community-service-secret",
	})
}

// NewWithConfig initializes the community router from runtime configuration.
// When DatabaseURL is configured, PostgreSQL stores backed by pgxpool are used; otherwise in-memory stores are used.
// Returned cleanup closes any open database resources.
func NewWithConfig(ctx context.Context, cfg *config.Config) (*gin.Engine, func(), error) {
	if cfg == nil {
		loaded, err := config.Load()
		if err != nil {
			return nil, nil, err
		}
		cfg = loaded
	}

	var (
		postStore     posts.Store
		interactStore interaction.Store
		modStore      moderation.Store
		notifStore    notification.Store
		cleanup       = func() {}
	)

	if cfg.DatabaseURL != "" {
		pool, err := openPostgresPool(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		postStore = posts.NewPostgresStore(pool)
		interactStore = interaction.NewPostgresStore(pool)
		modStore = moderation.NewPostgresStore(pool)
		notifStore = notification.NewPostgresStore(pool)
		cleanup = pool.Close
	} else {
		postStore = posts.NewMemoryStore()
		interactStore = interaction.NewMemoryStore()
		modStore = moderation.NewMemoryStore()
		notifStore = notification.NewMemoryStore()
	}

	httpTimeout := cfg.HTTPTimeout
	if httpTimeout <= 0 || httpTimeout > 10*time.Second {
		httpTimeout = 5 * time.Second
	}
	httpClient := &http.Client{Timeout: httpTimeout}

	identityToken := cfg.IdentityServiceToken
	if identityToken == "" {
		identityToken = cfg.ServiceToken
	}
	learningToken := cfg.LearningServiceToken
	if learningToken == "" {
		learningToken = cfg.ServiceToken
	}

	resolver := auth.NewIdentityResolver(cfg.IdentityServiceURL, identityToken, httpClient)
	courseChecker := auth.NewLearningAccessChecker(cfg.LearningServiceURL, learningToken, httpClient)

	router := NewWithDeps(Deps{
		SwaggerEnabled:      cfg.SwaggerEnabled,
		ServiceToken:        cfg.ServiceToken,
		PostStore:           postStore,
		InteractionStore:    interactStore,
		ModerationStore:     modStore,
		NotificationStore:   notifStore,
		SessionResolver:     resolver,
		CourseAccessChecker: courseChecker,
	})
	return router, cleanup, nil
}

// NewWithDeps builds a community router with explicitly injected dependencies (e.g., for integration tests).
func NewWithDeps(deps Deps) *gin.Engine {
	if deps.PostStore == nil {
		deps.PostStore = posts.NewMemoryStore()
	}
	if deps.InteractionStore == nil {
		deps.InteractionStore = interaction.NewMemoryStore()
	}
	if deps.ModerationStore == nil {
		deps.ModerationStore = moderation.NewMemoryStore()
	}
	if deps.NotificationStore == nil {
		deps.NotificationStore = notification.NewMemoryStore()
	}

	router := gin.New()
	router.Use(gin.Recovery())

	replyNotifier := func(ctx context.Context, eventID, recipientID, notifType, title, summary, targetType, targetID string) error {
		_, err := deps.NotificationStore.AppendNotification(ctx, eventID, recipientID, notifType, title, summary, targetType, targetID)
		return err
	}

	notifTargetChecker := func(ctx context.Context, userID, targetType, targetID string) bool {
		switch targetType {
		case "post":
			p, err := deps.PostStore.GetPost(ctx, targetID)
			if err != nil || p.Status != posts.StatusPublished {
				return false
			}
			if p.Scope == posts.ScopeCourse {
				if deps.CourseAccessChecker == nil {
					return false
				}
				allowed, err := deps.CourseAccessChecker(ctx, p.CourseID, userID)
				return err == nil && allowed
			}
			return true
		case "comment":
			cmt, err := deps.PostStore.GetComment(ctx, targetID)
			if err != nil || cmt.Status != posts.StatusPublished {
				return false
			}
			p, err := deps.PostStore.GetPost(ctx, cmt.PostID)
			if err != nil || p.Status != posts.StatusPublished {
				return false
			}
			if p.Scope == posts.ScopeCourse {
				if deps.CourseAccessChecker == nil {
					return false
				}
				allowed, err := deps.CourseAccessChecker(ctx, p.CourseID, userID)
				return err == nil && allowed
			}
			return true
		default:
			return true
		}
	}

	postsHandler := posts.NewHandler(deps.PostStore, deps.SessionResolver, deps.CourseAccessChecker, replyNotifier)
	interactHandler := interaction.NewHandler(deps.InteractionStore, deps.PostStore, deps.SessionResolver, deps.CourseAccessChecker)
	modHandler := moderation.NewHandler(deps.ModerationStore, deps.PostStore, deps.NotificationStore, deps.SessionResolver, deps.CourseAccessChecker)
	notifHandler := notification.NewHandler(deps.NotificationStore, deps.SessionResolver, deps.ServiceToken, notifTargetChecker)

	// Health
	router.GET("/api/community/health", health.Get)

	// Posts, Topics & Comments
	router.POST("/api/community/posts", postsHandler.CreatePost)
	router.GET("/api/community/posts", postsHandler.ListPosts)
	router.GET("/api/community/posts/:id", postsHandler.GetPost)
	router.PATCH("/api/community/posts/:id", postsHandler.UpdatePost)
	router.DELETE("/api/community/posts/:id", postsHandler.DeletePost)
	router.GET("/api/community/topics", postsHandler.ListTopics)
	router.POST("/api/community/posts/:id/comments", postsHandler.CreateComment)
	router.PATCH("/api/community/comments/:id", postsHandler.UpdateComment)
	router.DELETE("/api/community/comments/:id", postsHandler.DeleteComment)

	// Reactions & Favorites
	router.PUT("/api/community/reactions", interactHandler.SetReaction)
	router.GET("/api/community/reactions", interactHandler.GetReaction)
	router.PUT("/api/community/favorites", interactHandler.SetFavorite)
	router.GET("/api/community/favorites/:postId", interactHandler.GetFavoriteStatus)
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
		c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "not_found", Message: "route not found"})
	})

	if deps.SwaggerEnabled {
		router.GET("/swagger/community/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	return router
}
