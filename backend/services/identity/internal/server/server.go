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

// Options configures router behavior, security flags, mail delivery, and rate limiting.
type Options struct {
	SwaggerEnabled  bool
	ServiceToken    string
	CookieSecure    bool
	DevExposeTokens bool
	Mailer          accounts.Mailer
	AuthLimiter     *accounts.RateLimiter
	MailLimiter     *accounts.RateLimiter
}

// New returns an independent router with default in-memory stores and secure defaults.
func New(swaggerEnabled bool) *gin.Engine {
	accStore := accounts.NewMemoryStore()
	sessStore := sessions.NewMemoryStore()
	invStore := access.NewMemoryStore()
	return NewWithOptions(accStore, sessStore, invStore, Options{
		SwaggerEnabled: swaggerEnabled,
		ServiceToken:   "identity-service-secret",
	})
}

// NewWithStores returns a router wired with explicit stores and service token.
func NewWithStores(
	swaggerEnabled bool,
	accStore accounts.Store,
	sessStore sessions.Store,
	invStore access.Store,
	serviceToken string,
) *gin.Engine {
	return NewWithOptions(accStore, sessStore, invStore, Options{
		SwaggerEnabled: swaggerEnabled,
		ServiceToken:   serviceToken,
	})
}

// NewWithOptions returns a router wired with explicit stores and full options.
func NewWithOptions(
	accStore accounts.Store,
	sessStore sessions.Store,
	invStore access.Store,
	opts Options,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	accHandler := accounts.NewHandlerWithOptions(accStore, sessStore, accounts.Options{
		DevExposeTokens: opts.DevExposeTokens,
		Mailer:          opts.Mailer,
		AuthLimiter:     opts.AuthLimiter,
		MailLimiter:     opts.MailLimiter,
	})
	sessHandler := sessions.NewHandlerWithOptions(sessStore, accStore, sessions.Options{
		CookieSecure: opts.CookieSecure,
		AuthLimiter:  opts.AuthLimiter,
	})
	accessHandler := access.NewHandlerWithOptions(invStore, accStore, sessStore, access.Options{
		ServiceToken:    opts.ServiceToken,
		DevExposeTokens: opts.DevExposeTokens,
		Mailer:          opts.Mailer,
	})

	// Health
	router.GET("/api/identity/health", health.Get)

	// Accounts
	router.POST("/api/identity/accounts", accHandler.Register)
	router.GET("/api/identity/accounts", accessHandler.ListAccounts)
	router.PATCH("/api/identity/me", accHandler.UpdateMe)
	router.POST("/api/identity/email-verifications", accHandler.VerifyEmail)
	router.POST("/api/identity/email-verification-requests", accHandler.ResendEmailVerification)
	router.POST("/api/identity/password-reset-requests", accHandler.RequestPasswordReset)
	router.POST("/api/identity/password-resets", accHandler.ResetPassword)

	// Sessions
	router.POST("/api/identity/sessions", sessHandler.Login)
	router.DELETE("/api/identity/sessions/current", sessHandler.Logout)
	router.GET("/api/identity/me", sessHandler.GetCurrentUser)

	// Access & Administration
	router.GET("/api/identity/instructor-invitations", accessHandler.ListInstructorInvitations)
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

	if opts.SwaggerEnabled {
		router.GET("/swagger/identity/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	return router
}
