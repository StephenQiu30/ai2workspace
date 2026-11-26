package sessions

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	accountshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/accounts"
	accountsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/accounts"
	sessionsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/sessions"
	accountsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/accounts"
	"github.com/gin-gonic/gin"
)

// Options configures cookie security and rate limiting for session endpoints.
type Options struct {
	CookieSecure bool
	AuthLimiter  *accountshttp.RateLimiter
}

// Handler handles session endpoints.
type Handler struct {
	sessions     sessionsapp.Store
	accounts     accountsapp.Store
	cookieSecure bool
	authLimiter  *accountshttp.RateLimiter
}

// NewHandler initializes a session HTTP Handler with default options.
func NewHandler(sessions sessionsapp.Store, accounts accountsapp.Store) *Handler {
	return NewHandlerWithOptions(sessions, accounts, Options{})
}

// NewHandlerWithOptions initializes a session HTTP Handler with explicit options.
func NewHandlerWithOptions(sessions sessionsapp.Store, accounts accountsapp.Store, opts Options) *Handler {
	return &Handler{
		sessions:     sessions,
		accounts:     accounts,
		cookieSecure: opts.CookieSecure,
		authLimiter:  opts.AuthLimiter,
	}
}

func (h *Handler) setSessionCookie(c *gin.Context, rawToken string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionsapp.SessionCookieName,
		Value:    rawToken,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func toCurrentUserResponse(acc *accountsdomain.Account) CurrentUserResponse {
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
	c.JSON(http.StatusTooManyRequests, accountshttp.ErrorResponse{
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
// @Failure 400 {object} accountshttp.ErrorResponse
// @Failure 401 {object} accountshttp.ErrorResponse
// @Failure 403 {object} accountshttp.ErrorResponse
// @Failure 429 {object} accountshttp.ErrorResponse
// @Router /api/identity/sessions [post]
// @ID identityLogin
func (h *Handler) Login(c *gin.Context) {
	if allowed, retryAfter := h.authLimiter.Allow("login:" + c.ClientIP()); !allowed {
		writeRateLimitError(c, retryAfter)
		return
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_request",
			Message: "Email and password are required",
		})
		return
	}

	acc, err := h.accounts.Authenticate(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, accountsdomain.ErrAccountSuspended) {
			c.JSON(http.StatusForbidden, accountshttp.ErrorResponse{
				Error:   "account_suspended",
				Message: "Account has been suspended",
			})
			return
		}
		c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{
			Error:   "invalid_credentials",
			Message: "Incorrect email or password",
		})
		return
	}

	rawToken, _, err := h.sessions.CreateSession(c.Request.Context(), acc.ID, sessionsapp.DefaultSessionTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, accountshttp.ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to establish session",
		})
		return
	}

	h.setSessionCookie(c, rawToken, int(sessionsapp.DefaultSessionTTL.Seconds()))
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
	cookie, err := c.Cookie(sessionsapp.SessionCookieName)
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
// @Failure 401 {object} accountshttp.ErrorResponse
// @Failure 403 {object} accountshttp.ErrorResponse
// @Router /api/identity/me [get]
// @ID identityGetCurrentUser
func (h *Handler) GetCurrentUser(c *gin.Context) {
	cookie, err := c.Cookie(sessionsapp.SessionCookieName)
	if err != nil || cookie == "" {
		c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{
			Error:   "unauthorized",
			Message: "Authentication required",
		})
		return
	}

	sess, err := h.sessions.GetSession(c.Request.Context(), cookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{
			Error:   "session_invalid",
			Message: "Session expired or revoked",
		})
		return
	}

	acc, err := h.accounts.GetAccountByID(c.Request.Context(), sess.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{
			Error:   "account_not_found",
			Message: "Account no longer exists",
		})
		return
	}

	if acc.Status == accountsdomain.StatusSuspended {
		c.JSON(http.StatusForbidden, accountshttp.ErrorResponse{
			Error:   "account_suspended",
			Message: "Account has been suspended",
		})
		return
	}

	c.JSON(http.StatusOK, toCurrentUserResponse(acc))
}
