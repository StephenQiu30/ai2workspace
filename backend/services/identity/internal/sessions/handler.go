// Package sessions provides HTTP endpoints for session management.
package sessions

import (
	"errors"
	"net/http"

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
	UserID        string   `json:"userId" example:"usr_12345"`
	Email         string   `json:"email" example:"user@example.com"`
	EmailVerified bool     `json:"emailVerified" example:"true"`
	Roles         []string `json:"roles" example:"student"`
	AccountStatus string   `json:"accountStatus" example:"active"`
}

// Handler handles session endpoints.
type Handler struct {
	sessions Store
	accounts accounts.Store
}

// NewHandler initializes a session HTTP Handler.
func NewHandler(sessions Store, accounts accounts.Store) *Handler {
	return &Handler{
		sessions: sessions,
		accounts: accounts,
	}
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
// @Router /api/identity/sessions [post]
// @id identityLogin
func (h *Handler) Login(c *gin.Context) {
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

	c.SetCookie(SessionCookieName, rawToken, int(DefaultSessionTTL.Seconds()), "/", "", false, true)

	c.JSON(http.StatusOK, CurrentUserResponse{
		UserID:        acc.ID,
		Email:         acc.Email,
		EmailVerified: acc.EmailVerified,
		Roles:         acc.Roles,
		AccountStatus: acc.Status,
	})
}

// Logout godoc
// @Summary User logout
// @Description Revokes current session and clears session cookie.
// @Tags sessions
// @Produce json
// @Success 204 "No Content"
// @Router /api/identity/sessions/current [delete]
// @id identityLogout
func (h *Handler) Logout(c *gin.Context) {
	cookie, err := c.Cookie(SessionCookieName)
	if err == nil && cookie != "" {
		_ = h.sessions.RevokeSession(c.Request.Context(), cookie)
	}
	c.SetCookie(SessionCookieName, "", -1, "/", "", false, true)
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
// @id identityGetCurrentUser
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

	if acc.Status == "suspended" {
		c.JSON(http.StatusForbidden, accounts.ErrorResponse{
			Error:   "account_suspended",
			Message: "Account has been suspended",
		})
		return
	}

	c.JSON(http.StatusOK, CurrentUserResponse{
		UserID:        acc.ID,
		Email:         acc.Email,
		EmailVerified: acc.EmailVerified,
		Roles:         acc.Roles,
		AccountStatus: acc.Status,
	})
}
