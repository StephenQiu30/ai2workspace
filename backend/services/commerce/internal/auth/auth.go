// Package auth resolves authenticated principals via identity and verifies internal service tokens.
package auth

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// SessionCookieName is the shared opaque session cookie set by identity.
	SessionCookieName = "ai2_session"
	// ServiceTokenHeader is the shared internal service-to-service header.
	ServiceTokenHeader = "X-Service-Token"
)

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

type resolveSessionRequest struct {
	OpaqueSession string `json:"opaqueSession"`
}

// NewIdentityResolver builds a Resolver backed by identity's internal session resolve API.
func NewIdentityResolver(identityBaseURL, serviceToken string, httpClient *http.Client) Resolver {
	baseURL := strings.TrimRight(strings.TrimSpace(identityBaseURL), "/")
	if baseURL == "" {
		return DenyAllResolver
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	endpoint := baseURL + "/internal/identity/sessions/resolve"

	return func(c *gin.Context) (*Principal, error) {
		cookie, err := c.Cookie(SessionCookieName)
		if err != nil || strings.TrimSpace(cookie) == "" {
			return nil, ErrUnauthenticated
		}

		bodyBytes, err := json.Marshal(resolveSessionRequest{OpaqueSession: cookie})
		if err != nil {
			return nil, ErrIdentityUnavailable
		}

		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, ErrIdentityUnavailable
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(ServiceTokenHeader, serviceToken)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrIdentityUnavailable, err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			var principal Principal
			if err := json.NewDecoder(resp.Body).Decode(&principal); err != nil {
				return nil, fmt.Errorf("%w: invalid response payload", ErrIdentityUnavailable)
			}
			if principal.UserID == "" {
				return nil, ErrUnauthenticated
			}
			if principal.AccountStatus == "suspended" {
				return nil, ErrAccountSuspended
			}
			return &principal, nil
		case http.StatusUnauthorized:
			return nil, ErrUnauthenticated
		case http.StatusForbidden:
			return nil, ErrAccountSuspended
		default:
			return nil, fmt.Errorf("%w: status %d", ErrIdentityUnavailable, resp.StatusCode)
		}
	}
}

// VerifyServiceToken performs constant-time comparison of the X-Service-Token header.
func VerifyServiceToken(c *gin.Context, expected string) bool {
	if expected == "" {
		return false
	}
	provided := c.GetHeader(ServiceTokenHeader)
	if len(provided) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
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

// ContextWithTimeout ensures a bounded timeout for outbound calls without replacing parent cancellation.
func ContextWithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, d)
}
