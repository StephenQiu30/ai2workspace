package accounts

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	accountsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/accounts"
	accountsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/accounts"
	"github.com/gin-gonic/gin"
)

// SessionManager defines the subset of session capabilities needed by account actions.
type SessionManager interface {
	GetUserIDBySession(ctx context.Context, rawToken string) (string, error)
	RevokeAllForUser(ctx context.Context, userID string) error
}

// Options configures optional security and delivery behavior for accounts Handler.
type Options struct {
	DevExposeTokens bool
	Mailer          accountsapp.Mailer
	AuthLimiter     *RateLimiter
	MailLimiter     *RateLimiter
}

// Handler handles account registration, profile updates, email verification, and password reset.
type Handler struct {
	accounts        accountsapp.Store
	sessions        SessionManager
	devExposeTokens bool
	mailer          accountsapp.Mailer
	authLimiter     *RateLimiter
	mailLimiter     *RateLimiter
}

// NewHandler initializes an accounts Handler with secure defaults.
func NewHandler(accounts accountsapp.Store, sessions SessionManager) *Handler {
	return NewHandlerWithOptions(accounts, sessions, Options{})
}

// NewHandlerWithOptions initializes an accounts Handler with explicit options.
func NewHandlerWithOptions(accounts accountsapp.Store, sessions SessionManager, opts Options) *Handler {
	return &Handler{
		accounts:        accounts,
		sessions:        sessions,
		devExposeTokens: opts.DevExposeTokens,
		mailer:          opts.Mailer,
		authLimiter:     opts.AuthLimiter,
		mailLimiter:     opts.MailLimiter,
	}
}

func writeRateLimitError(c *gin.Context, retryAfter time.Duration) {
	secs := int(math.Ceil(retryAfter.Seconds()))
	if secs < 1 {
		secs = 1
	}
	c.Header("Retry-After", strconv.Itoa(secs))
	c.JSON(http.StatusTooManyRequests, ErrorResponse{
		Error:             "rate_limited",
		Message:           "Too many requests, please try again later",
		RetryAfterSeconds: secs,
	})
}

func (h *Handler) dispatchMail(ctx context.Context, recipient, subject string, purpose accountsdomain.TokenPurpose, rawToken, body string) error {
	if h.mailer == nil {
		_ = h.accounts.UpdateMailTaskStatus(ctx, recipient, purpose, rawToken, "sent")
		return nil
	}
	if err := h.mailer.Send(ctx, accountsapp.MailMessage{
		To:      recipient,
		Subject: subject,
		Body:    body,
	}); err != nil {
		_ = h.accounts.UpdateMailTaskStatus(ctx, recipient, purpose, rawToken, "failed")
		return err
	}
	_ = h.accounts.UpdateMailTaskStatus(ctx, recipient, purpose, rawToken, "sent")
	return nil
}

// Register godoc
// @Summary Register a new account
// @Description Registers a new user account with email, password, and optional nickname, creating an unverified account and dispatching a verification email.
// @Tags accounts
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Registration parameters"
// @Success 200 {object} RegisterResponse
// @Failure 400 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/identity/accounts [post]
// @ID identityRegister
func (h *Handler) Register(c *gin.Context) {
	if allowed, retryAfter := h.authLimiter.Allow("register:" + c.ClientIP()); !allowed {
		writeRateLimitError(c, retryAfter)
		return
	}

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Valid email and a password of at least 8 characters are required",
		})
		return
	}

	acc, token, err := h.accounts.CreateAccount(c.Request.Context(), req.Email, req.Password, req.Nickname)
	if err != nil {
		if errors.Is(err, accountsdomain.ErrDuplicateEmail) {
			// Uniform response to avoid email enumeration.
			c.JSON(http.StatusOK, RegisterResponse{
				Accepted: true,
				NextStep: "verify_email",
			})
			return
		}
		if errors.Is(err, accountsdomain.ErrInvalidNickname) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_nickname",
				Message: "Nickname must be between 1 and 64 characters",
			})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "registration_failed",
			Message: err.Error(),
		})
		return
	}

	mailBody := fmt.Sprintf("Welcome to ai2workspace. Use this verification token to verify your email: %s", token)
	if err := h.dispatchMail(c.Request.Context(), acc.Email, "Verify your email", accountsdomain.PurposeEmailVerification, token, mailBody); err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error:   "mail_delivery_failed",
			Message: "Account created, but verification email could not be delivered. Please retry sending verification.",
		})
		return
	}

	resp := RegisterResponse{
		Accepted: true,
		NextStep: "verify_email",
	}
	if h.devExposeTokens {
		resp.VerificationToken = token
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateMe godoc
// @Summary Update current user profile
// @Description Updates the currently authenticated user's nickname and increments account version.
// @Tags accounts
// @Accept json
// @Produce json
// @Param body body UpdateMeRequest true "Profile update payload"
// @Success 200 {object} ProfileResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/identity/me [patch]
// @ID identityUpdateMe
func (h *Handler) UpdateMe(c *gin.Context) {
	cookie, err := c.Cookie(accountsdomain.SessionCookieName)
	if err != nil || cookie == "" || h.sessions == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Authentication required",
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

	var req UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Nickname between 1 and 64 characters is required",
		})
		return
	}

	acc, err := h.accounts.UpdateNickname(c.Request.Context(), userID, req.Nickname)
	if err != nil {
		if errors.Is(err, accountsdomain.ErrInvalidNickname) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_nickname",
				Message: "Nickname must be between 1 and 64 non-whitespace characters",
			})
			return
		}
		if errors.Is(err, accountsdomain.ErrAccountSuspended) {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "account_suspended",
				Message: "Account has been suspended",
			})
			return
		}
		if errors.Is(err, accountsdomain.ErrAccountNotFound) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "account_not_found",
				Message: "Account not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update profile",
		})
		return
	}

	c.JSON(http.StatusOK, ProfileResponse{
		ID:            acc.ID,
		UserID:        acc.ID,
		Email:         acc.Email,
		Nickname:      acc.Nickname,
		EmailVerified: acc.EmailVerified,
		Status:        acc.Status,
		AccountStatus: acc.Status,
		Roles:         acc.Roles,
		Version:       acc.Version,
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
// @ID identityVerifyEmail
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
		if errors.Is(err, accountsdomain.ErrTokenExpired) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "token_expired",
				Message: "Verification token has expired",
			})
			return
		}
		if errors.Is(err, accountsdomain.ErrTokenConsumed) {
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
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/identity/email-verification-requests [post]
// @ID identityResendEmailVerification
func (h *Handler) ResendEmailVerification(c *gin.Context) {
	cookie, err := c.Cookie(accountsdomain.SessionCookieName)
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

	if acc.Status == accountsdomain.StatusSuspended {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "account_suspended",
			Message: "Account has been suspended",
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

	if allowed, retryAfter := h.mailLimiter.Allow("resend:" + acc.ID); !allowed {
		writeRateLimitError(c, retryAfter)
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

	mailBody := fmt.Sprintf("Use this verification token to verify your ai2workspace email: %s", rawToken)
	if err := h.dispatchMail(c.Request.Context(), acc.Email, "Verify your email", accountsdomain.PurposeEmailVerification, rawToken, mailBody); err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error:   "mail_delivery_failed",
			Message: "Failed to deliver verification email. Please retry later.",
		})
		return
	}

	resp := ResendVerificationResponse{
		Accepted: true,
		NextStep: "verify_email",
	}
	if h.devExposeTokens {
		resp.VerificationToken = rawToken
	}
	c.JSON(http.StatusOK, resp)
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
// @Failure 429 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/identity/password-reset-requests [post]
// @ID identityRequestPasswordReset
func (h *Handler) RequestPasswordReset(c *gin.Context) {
	if allowed, retryAfter := h.mailLimiter.Allow("pwdreset:" + c.ClientIP()); !allowed {
		writeRateLimitError(c, retryAfter)
		return
	}

	var req PasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Valid email is required",
		})
		return
	}

	token, err := h.accounts.CreatePasswordResetToken(c.Request.Context(), req.Email, 1*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "reset_request_failed",
			Message: "Failed to process password reset request",
		})
		return
	}

	if token != "" {
		normEmail := accountsdomain.NormalizeEmail(req.Email)
		mailBody := fmt.Sprintf("Use this token to reset your ai2workspace password: %s", token)
		if err := h.dispatchMail(c.Request.Context(), normEmail, "Reset your password", accountsdomain.PurposePasswordReset, token, mailBody); err != nil {
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{
				Error:   "mail_delivery_failed",
				Message: "Failed to deliver password reset email. Please retry later.",
			})
			return
		}
	}

	// Always return accepted to prevent email enumeration.
	resp := PasswordResetResponse{
		Accepted: true,
	}
	if h.devExposeTokens && token != "" {
		resp.ResetToken = token
	}
	c.JSON(http.StatusOK, resp)
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
// @ID identityResetPassword
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
		if errors.Is(err, accountsdomain.ErrTokenExpired) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "token_expired",
				Message: "Reset token has expired",
			})
			return
		}
		if errors.Is(err, accountsdomain.ErrTokenConsumed) {
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
