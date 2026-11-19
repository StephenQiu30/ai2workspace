// Package server assembles the service's HTTP routes.
package server

import (
	"net/http"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/identity/api"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/access"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/accounts"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/health"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/sessions"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// New returns an independent router with default in-memory stores.
func New(swaggerEnabled bool) *gin.Engine {
	accStore := accounts.NewMemoryStore()
	sessStore := sessions.NewMemoryStore()
	invStore := access.NewMemoryStore()
	return NewWithStores(swaggerEnabled, accStore, sessStore, invStore, "identity-service-secret")
}

// NewWithStores returns a router wired with explicit stores and service configuration.
func NewWithStores(
	swaggerEnabled bool,
	accStore accounts.Store,
	sessStore sessions.Store,
	invStore access.Store,
	serviceToken string,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	accHandler := accounts.NewHandler(accStore, sessStore)
	sessHandler := sessions.NewHandler(sessStore, accStore)
	accessHandler := access.NewHandler(invStore, accStore, sessStore, serviceToken)

	// Health
	router.GET("/api/identity/health", health.Get)

	// Accounts
	router.POST("/api/identity/accounts", accHandler.Register)
	router.POST("/api/identity/email-verifications", accHandler.VerifyEmail)
	router.POST("/api/identity/email-verification-requests", accHandler.ResendEmailVerification)
	router.POST("/api/identity/password-reset-requests", accHandler.RequestPasswordReset)
	router.POST("/api/identity/password-resets", accHandler.ResetPassword)

	// Sessions
	router.POST("/api/identity/sessions", sessHandler.Login)
	router.DELETE("/api/identity/sessions/current", sessHandler.Logout)
	router.GET("/api/identity/me", sessHandler.GetCurrentUser)

	// Access & Administration
	router.POST("/api/identity/instructor-invitations", accessHandler.InviteInstructor)
	router.POST("/api/identity/instructor-invitations/accept", accessHandler.AcceptInstructorInvitation)
	router.DELETE("/api/identity/instructor-invitations/:inviteId", accessHandler.CancelInstructorInvitation)
	router.PATCH("/api/identity/accounts/:userId/status", accessHandler.SetAccountStatus)
	router.DELETE("/api/identity/accounts/:userId/instructor-role", accessHandler.RevokeInstructor)

	// Internal Service Resolution
	router.POST("/internal/identity/sessions/resolve", accessHandler.ResolveSession)

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	})

	if swaggerEnabled {
		router.GET("/swagger/identity/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	return router
}
