package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const ()

var (
	ErrUnauthenticated     = errors.New("authentication required")
	ErrAccountSuspended    = errors.New("account is suspended")
	ErrEmailNotVerified    = errors.New("email verification required")
	ErrForbidden           = errors.New("permission denied")
	ErrIdentityUnavailable = errors.New("identity service unavailable")
)

// Principal represents a verified caller resolved from identity.
type Principal struct {
	UserID         string   `json:"userId"`
	Email          string   `json:"email"`
	EmailVerified  bool     `json:"emailVerified"`
	Roles          []string `json:"roles"`
	AccountStatus  string   `json:"accountStatus"`
	SessionVersion int      `json:"sessionVersion"`
}

// HasRole reports whether the principal carries the given role.
func (p *Principal) HasRole(role string) bool {
	if p == nil {
		return false
	}
	for _, r := range p.Roles {
		if strings.EqualFold(r, role) {
			return true
		}
	}
	return false
}

// IsAdmin reports whether the principal has admin privileges.
func (p *Principal) IsAdmin() bool {
	return p.HasRole("admin")
}

// Resolver resolves the authenticated principal from an incoming Gin request.
type Resolver func(c *gin.Context) (*Principal, error)

// DenyAllResolver rejects all requests when identity resolution is not configured.
func DenyAllResolver(_ *gin.Context) (*Principal, error) {
	return nil, ErrUnauthenticated
}

// ResolvePrincipal resolves the caller and writes an appropriate HTTP error response if unsuccessful.
func ResolvePrincipal(c *gin.Context, resolver Resolver) (*Principal, bool) {
	if resolver == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": ErrUnauthenticated.Error()})
		return nil, false
	}
	principal, err := resolver(c)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnauthenticated):
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case errors.Is(err, ErrAccountSuspended), errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrIdentityUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusUnauthorized, gin.H{"error": ErrUnauthenticated.Error()})
		}
		return nil, false
	}
	if principal == nil || principal.UserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": ErrUnauthenticated.Error()})
		return nil, false
	}
	if principal.AccountStatus == "suspended" {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrAccountSuspended.Error()})
		return nil, false
	}
	return principal, true
}
