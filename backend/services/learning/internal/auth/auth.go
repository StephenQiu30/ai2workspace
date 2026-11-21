// Package auth defines trusted caller identity types and Gin authentication middleware.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// SessionCookieName is the trusted opaque session cookie issued by identity.
	SessionCookieName = "ai2_session"

	principalContextKey = "learning.auth.principal"
	authErrContextKey   = "learning.auth.error"
)

var (
	// ErrUnauthenticated indicates missing, expired, or invalid session credentials.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrAccountSuspended indicates the account is suspended and cannot access protected resources.
	ErrAccountSuspended = errors.New("account suspended")
	// ErrAuthUnavailable indicates the identity service could not be reached to validate the session.
	ErrAuthUnavailable = errors.New("authentication service unavailable")
)

// Principal represents a verified caller resolved from an opaque session.
type Principal struct {
	UserID         string   `json:"userId"`
	Email          string   `json:"email"`
	Nickname       string   `json:"nickname"`
	EmailVerified  bool     `json:"emailVerified"`
	Roles          []string `json:"roles"`
	Status         string   `json:"accountStatus"`
	SessionVersion int      `json:"sessionVersion"`
}

// HasRole reports whether the principal holds the given role (case-insensitive).
func (p *Principal) HasRole(role string) bool {
	if p == nil {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(role))
	for _, r := range p.Roles {
		if strings.ToLower(strings.TrimSpace(r)) == target {
			return true
		}
	}
	return false
}

// IsAdmin reports whether the principal has administrator privileges.
func (p *Principal) IsAdmin() bool {
	return p.HasRole("admin")
}

// IsInstructor reports whether the principal has instructor privileges.
func (p *Principal) IsInstructor() bool {
	return p.HasRole("instructor")
}

// IsInstructorOrAdmin reports whether the principal can author courses.
func (p *Principal) IsInstructorOrAdmin() bool {
	return p.IsInstructor() || p.IsAdmin()
}

// IsActive reports whether the account is not suspended/disabled.
func (p *Principal) IsActive() bool {
	if p == nil {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(p.Status))
	return status == "" || status == "active"
}

// DisplayName returns the best available public display name for the principal.
func (p *Principal) DisplayName() string {
	if p == nil {
		return ""
	}
	if nick := strings.TrimSpace(p.Nickname); nick != "" {
		return nick
	}
	if email := strings.TrimSpace(p.Email); email != "" {
		if at := strings.Index(email, "@"); at > 0 {
			return email[:at]
		}
		return email
	}
	return p.UserID
}

// Authenticator resolves an HTTP request into a verified Principal.
// Returning (nil, nil) represents an unauthenticated guest request.
type Authenticator func(ctx context.Context, r *http.Request) (*Principal, error)

// Middleware resolves request identity once and stores the result in Gin context.
// It never aborts public requests; protected handlers enforce authentication via RequirePrincipal.
func Middleware(authenticator Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authenticator != nil {
			principal, err := authenticator(c.Request.Context(), c.Request)
			if err != nil {
				c.Set(authErrContextKey, err)
			} else if principal != nil {
				c.Set(principalContextKey, principal)
			}
		}
		c.Next()
	}
}

// FromContext retrieves the resolved Principal and any authentication error from Gin context.
func FromContext(c *gin.Context) (*Principal, error) {
	if c == nil {
		return nil, nil
	}
	if rawErr, exists := c.Get(authErrContextKey); exists {
		if err, ok := rawErr.(error); ok && err != nil {
			return nil, err
		}
	}
	if rawPrincipal, exists := c.Get(principalContextKey); exists {
		if p, ok := rawPrincipal.(*Principal); ok && p != nil {
			return p, nil
		}
	}
	return nil, nil
}

// RequirePrincipal enforces that the request carries a valid, active Principal.
// On failure it writes an appropriate 401/403/503 JSON error and returns (nil, false).
func RequirePrincipal(c *gin.Context) (*Principal, bool) {
	p, err := FromContext(c)
	if err != nil {
		switch {
		case errors.Is(err, ErrAccountSuspended):
			c.JSON(http.StatusForbidden, gin.H{"error": "account_suspended", "message": "Account has been suspended"})
		case errors.Is(err, ErrAuthUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth_unavailable", "message": "Authentication service temporarily unavailable"})
		default:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Authentication required"})
		}
		return nil, false
	}
	if p == nil || p.UserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Authentication required"})
		return nil, false
	}
	if !p.IsActive() {
		c.JSON(http.StatusForbidden, gin.H{"error": "account_suspended", "message": "Account has been suspended"})
		return nil, false
	}
	return p, true
}
