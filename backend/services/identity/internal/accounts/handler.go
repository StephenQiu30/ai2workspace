// Package accounts provides HTTP handlers for account management.
package accounts

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const SessionCookieName = "ai2_session"

// SessionManager defines the subset of session capabilities needed by account actions.
type SessionManager interface {
	GetUserIDBySession(ctx context.Context, rawToken string) (string, error)
	RevokeAllForUser(ctx context.Context, userID string) error
}

// Handler handles account registration, email verification, and password reset.
type Handler struct {
	accounts Store
	sessions SessionManager
}

// NewHandler initializes an accounts Handler.
func NewHandler(accounts Store, sessions SessionManager) *Handler {
	return &Handler{
		accounts: accounts,
		sessions: sessions,
	}
}

// Register godoc
// @Summary Register a new account
// @Description Registers a new user account with email and password, creating an unverified account and sending a verification email.
// @Tags accounts
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Registration parameters"
// @Success 200 {object} RegisterResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/identity/accounts [post]
// @id identityRegister
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Valid email and a password of at least 8 characters are required",
		})
		return
	}

	_, token, err := h.accounts.CreateAccount(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			// Uniform response to avoid email enumeration
			c.JSON(http.StatusOK, RegisterResponse{
				Accepted: true,
				NextStep: "verify_email",
			})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "registration_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, RegisterResponse{
		Accepted:          true,
		NextStep:          "verify_email",
		VerificationToken: token,
	})
}

// VerifyEmail godoc
// @Summary Verify email with token
// @Description Consumes a single-use verification token to mark the user email as verified.
// @Tags accounts
// @Accept json
// @Produce json
// @Param body body VerifyEmailRequest true "Verification token"
// @Success 200 {object} VerifyEmailResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/identity/email-verifications [post]
// @id identityVerifyEmail
func (h *Handler) VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Verification token is required",
		})
		return
	}

	acc, err := h.accounts.VerifyEmailWithToken(c.Request.Context(), req.Token)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "token_expired",
				Message: "Verification token has expired",
			})
			return
		}
		if errors.Is(err, ErrTokenConsumed) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "token_consumed",
				Message: "Verification token has already been used",
			})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_token",
			Message: "Invalid verification token",
		})
		return
	}

	c.JSON(http.StatusOK, VerifyEmailResponse{
		EmailVerified: acc.EmailVerified,
	})
}

// ResendEmailVerification godoc
// @Summary Resend verification email
// @Description Re-sends a verification email for the currently authenticated unverified account.
// @Tags accounts
// @Produce json
// @Success 200 {object} ResendVerificationResponse
// @Failure 401 {object} ErrorResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/identity/email-verification-requests [post]
// @id identityResendEmailVerification
func (h *Handler) ResendEmailVerification(c *gin.Context) {
	cookie, err := c.Cookie(SessionCookieName)
	if err != nil || cookie == "" || h.sessions == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Active session required to resend verification",
		})
		return
	}

	userID, err := h.sessions.GetUserIDBySession(c.Request.Context(), cookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "session_invalid",
			Message: "Invalid or expired session",
		})
		return
	}

	acc, err := h.accounts.GetAccountByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "account_not_found",
			Message: "Account not found",
		})
		return
	}

	if acc.EmailVerified {
		c.JSON(http.StatusOK, ResendVerificationResponse{
			Accepted: true,
			NextStep: "already_verified",
		})
		return
	}

	rawToken, err := h.accounts.CreateVerificationToken(c.Request.Context(), acc.Email, 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "send_failed",
			Message: "Failed to dispatch verification email",
		})
		return
	}

	c.JSON(http.StatusOK, ResendVerificationResponse{
		Accepted:          true,
		NextStep:          "verify_email",
		VerificationToken: rawToken,
	})
}

// RequestPasswordReset godoc
// @Summary Request password reset
// @Description Requests a password reset link sent to the registered email.
// @Tags accounts
// @Accept json
// @Produce json
// @Param body body PasswordResetRequest true "Email for password reset"
// @Success 200 {object} PasswordResetResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/identity/password-reset-requests [post]
// @id identityRequestPasswordReset
func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var req PasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Valid email is required",
		})
		return
	}

	token, _ := h.accounts.CreatePasswordResetToken(c.Request.Context(), req.Email, 1*time.Hour)

	// Always return accepted to prevent enumeration
	c.JSON(http.StatusOK, PasswordResetResponse{
		Accepted:   true,
		ResetToken: token,
	})
}

// ResetPassword godoc
// @Summary Reset password
// @Description Updates password using a valid reset token and invalidates existing user sessions.
// @Tags accounts
// @Accept json
// @Produce json
// @Param body body ResetPasswordRequest true "Reset token and new password"
// @Success 200 {object} ResetPasswordResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/identity/password-resets [post]
// @id identityResetPassword
func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Token and a new password with at least 8 characters are required",
		})
		return
	}

	acc, err := h.accounts.ResetPasswordWithToken(c.Request.Context(), req.Token, req.NewPassword)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "token_expired",
				Message: "Reset token has expired",
			})
			return
		}
		if errors.Is(err, ErrTokenConsumed) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "token_consumed",
				Message: "Reset token has already been used",
			})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_token",
			Message: "Invalid reset token",
		})
		return
	}

	if h.sessions != nil {
		_ = h.sessions.RevokeAllForUser(c.Request.Context(), acc.ID)
	}

	c.JSON(http.StatusOK, ResetPasswordResponse{
		Updated: true,
	})
}
