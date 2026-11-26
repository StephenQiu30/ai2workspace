// Package http binds endpoints for the identity module.
package http

import (
	accesshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/access"
	accountshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/accounts"
	sessionshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/sessions"
	"github.com/gin-gonic/gin"
)

// Routes holds the HTTP handlers for the identity module.
type Routes struct {
	Accounts *accountshttp.Handler
	Sessions *sessionshttp.Handler
	Access   *accesshttp.Handler
}

// RegisterRoutes adds identity endpoints to the application router.
func (r Routes) RegisterRoutes(router gin.IRouter) {
	// Accounts
	router.POST("/api/identity/accounts", r.Accounts.Register)
	router.GET("/api/identity/accounts", r.Access.ListAccounts)
	router.PATCH("/api/identity/me", r.Accounts.UpdateMe)
	router.POST("/api/identity/email-verifications", r.Accounts.VerifyEmail)
	router.POST("/api/identity/email-verification-requests", r.Accounts.ResendEmailVerification)
	router.POST("/api/identity/password-reset-requests", r.Accounts.RequestPasswordReset)
	router.POST("/api/identity/password-resets", r.Accounts.ResetPassword)

	// Sessions
	router.POST("/api/identity/sessions", r.Sessions.Login)
	router.DELETE("/api/identity/sessions/current", r.Sessions.Logout)
	router.GET("/api/identity/me", r.Sessions.GetCurrentUser)

	// Access & Administration
	router.GET("/api/identity/instructor-invitations", r.Access.ListInstructorInvitations)
	router.POST("/api/identity/instructor-invitations", r.Access.InviteInstructor)
	router.POST("/api/identity/instructor-invitations/accept", r.Access.AcceptInstructorInvitation)
	router.DELETE("/api/identity/instructor-invitations/:inviteId", r.Access.CancelInstructorInvitation)
	router.PATCH("/api/identity/accounts/:userId/status", r.Access.SetAccountStatus)
	router.DELETE("/api/identity/accounts/:userId/instructor-role", r.Access.RevokeInstructor)
}
