// Package sessions provides HTTP endpoints for session management.
package sessions

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/accounts"
	"github.com/gin-gonic/gin"
)

// LoginRequest defines credentials for authentication.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com"`
	Password string `json:"password" binding:"required" example:"SecureP@ss123"`
}

// CurrentUserResponse returns the authenticated user profile.
type CurrentUserResponse struct {
	ID            string   `json:"id" example:"usr_12345"`
	UserID        string   `json:"userId" example:"usr_12345"`
	Email         string   `json:"email" example:"user@example.com"`
	Nickname      string   `json:"nickname" example:"alice"`
	EmailVerified bool     `json:"emailVerified" example:"true"`
	Status        string   `json:"status" example:"active"`
	AccountStatus string   `json:"accountStatus" example:"active"`
	Roles         []string `json:"roles" example:"student"`
	Version       int      `json:"version" example:"1"`
}

// Options configures cookie security and rate limiting for session endpoints.
type Options struct {
	CookieSecure bool
	AuthLimiter  *accounts.RateLimiter
}

// Handler handles session endpoints.
type Handler struct {
	sessions     Store
	accounts     accounts.Store
	cookieSecure bool
	authLimiter  *accounts.RateLimiter
}

// NewHandler initializes a session HTTP Handler with default options.
func NewHandler(sessions Store, accounts accounts.Store) *Handler {
	return NewHandlerWithOptions(sessions, accounts, Options{})
}

// NewHandlerWithOptions initializes a session HTTP Handler with explicit options.
func NewHandlerWithOptions(sessions Store, accounts accounts.Store, opts Options) *Handler {
	return &Handler{
		sessions:     sessions,
		accounts:     accounts,
		cookieSecure: opts.CookieSecure,
		authLimiter:  opts.AuthLimiter,
	}
}

func (h *Handler) setSessionCookie(c *gin.Context, rawToken string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     SessionCookieName,
		Value:    rawToken,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func toCurrentUserResponse(acc *accounts.Account) CurrentUserResponse {
	return CurrentUserResponse{
		ID:            acc.ID,
		UserID:        acc.ID,
		Email:         acc.Email,
		Nickname:      acc.Nickname,
		EmailVerified: acc.EmailVerified,
		Status:        acc.Status,
		AccountStatus: acc.Status,
		Roles:         acc.Roles,
		Version:       acc.Version,
	}
}

func writeRateLimitError(c *gin.Context, retryAfter time.Duration) {
	secs := int(math.Ceil(retryAfter.Seconds()))
	if secs < 1 {
		secs = 1
	}
	c.Header("Retry-After", strconv.Itoa(secs))
	c.JSON(http.StatusTooManyRequests, accounts.ErrorResponse{
		Error:             "rate_limited",
		Message:           "Too many login attempts, please try again later",
		RetryAfterSeconds: secs,
	})
}

// Login godoc
// @Summary User login
// @Description Authenticates email and password, returning user profile and setting session cookie.
// @Tags sessions
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Login credentials"
// @Success 200 {object} CurrentUserResponse
// @Failure 400 {object} accounts.ErrorResponse
// @Failure 401 {object} accounts.ErrorResponse
// @Failure 403 {object} accounts.ErrorResponse
// @Failure 429 {object} accounts.ErrorResponse
// @Router /api/identity/sessions [post]
// @ID identityLogin
func (h *Handler) Login(c *gin.Context) {
	if allowed, retryAfter := h.authLimiter.Allow("login:" + c.ClientIP()); !allowed {
		writeRateLimitError(c, retryAfter)
		return
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "invalid_request",
			Message: "Email and password are required",
		})
		return
	}

	acc, err := h.accounts.Authenticate(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, accounts.ErrAccountSuspended) {
			c.JSON(http.StatusForbidden, accounts.ErrorResponse{
				Error:   "account_suspended",
				Message: "Account has been suspended",
			})
			return
		}
		c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{
			Error:   "invalid_credentials",
			Message: "Incorrect email or password",
		})
		return
	}

	rawToken, _, err := h.sessions.CreateSession(c.Request.Context(), acc.ID, DefaultSessionTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, accounts.ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to establish session",
		})
		return
	}

	h.setSessionCookie(c, rawToken, int(DefaultSessionTTL.Seconds()))
	c.JSON(http.StatusOK, toCurrentUserResponse(acc))
}

// Logout godoc
// @Summary User logout
// @Description Revokes current session and clears session cookie.
// @Tags sessions
// @Produce json
// @Success 204 "No Content"
// @Router /api/identity/sessions/current [delete]
// @ID identityLogout
func (h *Handler) Logout(c *gin.Context) {
	cookie, err := c.Cookie(SessionCookieName)
	if err == nil && cookie != "" {
		_ = h.sessions.RevokeSession(c.Request.Context(), cookie)
	}
	h.setSessionCookie(c, "", -1)
	c.Status(http.StatusNoContent)
}

// GetCurrentUser godoc
// @Summary Get current user
// @Description Returns the profile of the currently authenticated user based on session cookie.
// @Tags sessions
// @Produce json
// @Success 200 {object} CurrentUserResponse
// @Failure 401 {object} accounts.ErrorResponse
// @Failure 403 {object} accounts.ErrorResponse
// @Router /api/identity/me [get]
// @ID identityGetCurrentUser
func (h *Handler) GetCurrentUser(c *gin.Context) {
	cookie, err := c.Cookie(SessionCookieName)
	if err != nil || cookie == "" {
		c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{
			Error:   "unauthorized",
			Message: "Authentication required",
		})
		return
	}

	sess, err := h.sessions.GetSession(c.Request.Context(), cookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{
			Error:   "session_invalid",
			Message: "Session expired or revoked",
		})
		return
	}

	acc, err := h.accounts.GetAccountByID(c.Request.Context(), sess.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{
			Error:   "account_not_found",
			Message: "Account no longer exists",
		})
		return
	}

	if acc.Status == accounts.StatusSuspended {
		c.JSON(http.StatusForbidden, accounts.ErrorResponse{
			Error:   "account_suspended",
			Message: "Account has been suspended",
		})
		return
	}

	c.JSON(http.StatusOK, toCurrentUserResponse(acc))
}
