package assistant

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/gin-gonic/gin"
)

var (
	ErrUnauthorized     = errors.New("valid authenticated session required")
	ErrAccountSuspended = errors.New("account is suspended")
	ErrForbidden        = errors.New("insufficient permissions")
	ErrEmailNotVerified = errors.New("verified email is required to use assistant chat")
)

// Actor represents a verified caller resolved from an opaque session via the identity service.
type Actor struct {
	UserID        string   `json:"userId"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"emailVerified"`
	Roles         []string `json:"roles"`
	AccountStatus string   `json:"accountStatus"`
}

// HasRole checks whether the actor holds the given role.
func (a *Actor) HasRole(role string) bool {
	if a == nil {
		return false
	}
	for _, r := range a.Roles {
		if strings.EqualFold(strings.TrimSpace(r), role) {
			return true
		}
	}
	return false
}

// IsAdmin returns true when the actor holds the admin role.
func (a *Actor) IsAdmin() bool {
	return a.HasRole("admin")
}

// IsInstructor returns true when the actor holds the instructor role.
func (a *Actor) IsInstructor() bool {
	return a.HasRole("instructor")
}

// CanManageKnowledge returns true when the actor is an admin or instructor.
func (a *Actor) CanManageKnowledge() bool {
	return a.IsAdmin() || a.IsInstructor()
}

// SessionResolver resolves an opaque ai2_session cookie into a verified Actor.
type SessionResolver func(ctx context.Context, opaqueSession string) (*Actor, error)

func (m *Handler) authenticate(c *gin.Context) (*Actor, bool) {
	cookie, err := c.Cookie(identity.SessionCookieName)
	if err != nil || strings.TrimSpace(cookie) == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Code: http.StatusUnauthorized, Message: ErrUnauthorized.Error()})
		return nil, false
	}
	actor, err := m.sessionResolver(c.Request.Context(), cookie)
	if err != nil {
		if errors.Is(err, ErrAccountSuspended) {
			c.JSON(http.StatusForbidden, ErrorResponse{Code: http.StatusForbidden, Message: ErrAccountSuspended.Error()})
			return nil, false
		}
		c.JSON(http.StatusUnauthorized, ErrorResponse{Code: http.StatusUnauthorized, Message: ErrUnauthorized.Error()})
		return nil, false
	}
	if actor == nil || strings.TrimSpace(actor.UserID) == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Code: http.StatusUnauthorized, Message: ErrUnauthorized.Error()})
		return nil, false
	}
	if strings.EqualFold(actor.AccountStatus, "suspended") {
		c.JSON(http.StatusForbidden, ErrorResponse{Code: http.StatusForbidden, Message: ErrAccountSuspended.Error()})
		return nil, false
	}
	return actor, true
}

func (m *Handler) requireAdmin(c *gin.Context) (*Actor, bool) {
	actor, ok := m.authenticate(c)
	if !ok {
		return nil, false
	}
	if !actor.IsAdmin() {
		c.JSON(http.StatusForbidden, ErrorResponse{Code: http.StatusForbidden, Message: "admin role required"})
		return nil, false
	}
	return actor, true
}

func (m *Handler) requireKnowledgeMaintainer(c *gin.Context) (*Actor, bool) {
	actor, ok := m.authenticate(c)
	if !ok {
		return nil, false
	}
	if !actor.CanManageKnowledge() {
		c.JSON(http.StatusForbidden, ErrorResponse{Code: http.StatusForbidden, Message: "admin or instructor role required"})
		return nil, false
	}
	return actor, true
}

func (m *Handler) requireVerifiedLearner(c *gin.Context) (*Actor, bool) {
	actor, ok := m.authenticate(c)
	if !ok {
		return nil, false
	}
	if !actor.EmailVerified {
		c.JSON(http.StatusForbidden, ErrorResponse{Code: http.StatusForbidden, Message: ErrEmailNotVerified.Error()})
		return nil, false
	}
	return actor, true
}
