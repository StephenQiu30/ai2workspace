package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/gin-gonic/gin"
)

const ()

var (
	// ErrUnauthenticated indicates missing, invalid, or expired session credentials.
	ErrUnauthenticated = errors.New("authentication required")
	// ErrAccountSuspended indicates the account is suspended and cannot act.
	ErrAccountSuspended = errors.New("account suspended")
	// ErrEmailNotVerified indicates email verification is required for community actions.
	ErrEmailNotVerified = errors.New("email verification required")
	// ErrForbidden indicates insufficient privileges.
	ErrForbidden = errors.New("forbidden")
	// ErrDependencyUnavailable indicates an upstream auth/access dependency failed.
	ErrDependencyUnavailable = errors.New("upstream authorization service unavailable")
)

// ErrorResponse is the standard error envelope returned by community HTTP handlers.
type ErrorResponse struct {
	Error   string `json:"error" example:"unauthorized"`
	Message string `json:"message,omitempty" example:"authentication required"`
}

// Actor represents an authenticated caller resolved from identity.
type Actor struct {
	UserID        string   `json:"userId"`
	Nickname      string   `json:"nickname"`
	Email         string   `json:"email,omitempty"`
	EmailVerified bool     `json:"emailVerified"`
	Roles         []string `json:"roles"`
	AccountStatus string   `json:"accountStatus"`
}

// DisplayName returns a safe public display name for the actor.
func (a *Actor) DisplayName() string {
	if a == nil {
		return ""
	}
	if name := strings.TrimSpace(a.Nickname); name != "" {
		return name
	}
	if email := strings.TrimSpace(a.Email); email != "" {
		if idx := strings.Index(email, "@"); idx > 0 {
			return email[:idx]
		}
		return email
	}
	return a.UserID
}

// HasRole checks whether the actor holds the given role.
func (a *Actor) HasRole(role string) bool {
	if a == nil {
		return false
	}
	for _, r := range a.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAdmin returns true when the actor holds the admin role.
func (a *Actor) IsAdmin() bool {
	return a.HasRole("admin")
}

// IsSuspended returns true when the actor's account is suspended.
func (a *Actor) IsSuspended() bool {
	return a != nil && strings.EqualFold(a.AccountStatus, "suspended")
}

// SessionResolver resolves an opaque ai2_session cookie into a verified Actor.
type SessionResolver func(ctx context.Context, opaqueSession string) (*Actor, error)

// CourseAccessChecker verifies whether userID currently has discussion access to courseID.
type CourseAccessChecker func(ctx context.Context, courseID, userID string) (bool, error)

// ResolveActor strictly resolves an authenticated caller from the ai2_session cookie.
// It never falls back to headers or mock identities.
func ResolveActor(c *gin.Context, resolver SessionResolver) (*Actor, error) {
	cookie, err := c.Cookie(identity.SessionCookieName)
	if err != nil || strings.TrimSpace(cookie) == "" {
		return nil, ErrUnauthenticated
	}
	if resolver == nil {
		return nil, ErrDependencyUnavailable
	}
	actor, err := resolver(c.Request.Context(), cookie)
	if err != nil {
		return nil, err
	}
	if actor == nil || strings.TrimSpace(actor.UserID) == "" {
		return nil, ErrUnauthenticated
	}
	if actor.IsSuspended() {
		return nil, ErrAccountSuspended
	}
	return actor, nil
}

// OptionalActor attempts to resolve the caller for endpoints that also support guest reads.
// Missing or invalid sessions resolve to (nil, nil) so guests only receive public fields.
func OptionalActor(c *gin.Context, resolver SessionResolver) (*Actor, error) {
	cookie, err := c.Cookie(identity.SessionCookieName)
	if err != nil || strings.TrimSpace(cookie) == "" {
		return nil, nil
	}
	if resolver == nil {
		return nil, nil
	}
	actor, err := resolver(c.Request.Context(), cookie)
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			return nil, nil
		}
		return nil, err
	}
	if actor == nil || strings.TrimSpace(actor.UserID) == "" {
		return nil, nil
	}
	if actor.IsSuspended() {
		return nil, ErrAccountSuspended
	}
	return actor, nil
}

// WriteAuthError writes the HTTP error response corresponding to an auth error.
func WriteAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUnauthenticated):
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "authentication required"})
	case errors.Is(err, ErrAccountSuspended):
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "account_suspended", Message: "account has been suspended"})
	case errors.Is(err, ErrEmailNotVerified):
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "email_not_verified", Message: "email verification is required"})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "insufficient permissions"})
	case errors.Is(err, ErrDependencyUnavailable):
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "service_unavailable", Message: "authorization service unavailable"})
	default:
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: err.Error()})
	}
}

// RequireAuthenticated enforces an active authenticated session.
func RequireAuthenticated(c *gin.Context, resolver SessionResolver) (*Actor, bool) {
	actor, err := ResolveActor(c, resolver)
	if err != nil {
		WriteAuthError(c, err)
		return nil, false
	}
	return actor, true
}

// RequireVerified enforces an active authenticated session with a verified email address.
func RequireVerified(c *gin.Context, resolver SessionResolver) (*Actor, bool) {
	actor, ok := RequireAuthenticated(c, resolver)
	if !ok {
		return nil, false
	}
	if !actor.EmailVerified {
		WriteAuthError(c, ErrEmailNotVerified)
		return nil, false
	}
	return actor, true
}

// RequireAdmin enforces an active authenticated session with the admin role.
func RequireAdmin(c *gin.Context, resolver SessionResolver) (*Actor, bool) {
	actor, ok := RequireAuthenticated(c, resolver)
	if !ok {
		return nil, false
	}
	if !actor.IsAdmin() {
		WriteAuthError(c, ErrForbidden)
		return nil, false
	}
	return actor, true
}
