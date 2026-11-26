// Package application implements the creation module's workspace use cases.
package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/StephenQiu30/ai2workspace/backend/internal/creation/domain"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
)

var (
	// ErrIdentityUnavailable indicates that session validation failed.
	ErrIdentityUnavailable = errors.New("identity unavailable")
	// ErrWorkspaceUnavailable indicates that membership validation failed.
	ErrWorkspaceUnavailable = errors.New("workspace access unavailable")
)

// Memberships gets active roles from the workspace membership records.
type Memberships interface {
	ActiveRole(ctx context.Context, workspaceID, userID string) (domain.Role, error)
}

// Access checks the current identity and workspace membership.
type Access struct {
	resolve identity.SessionResolver
	members Memberships
}

// NewAccess builds the workspace access use case.
func NewAccess(resolve identity.SessionResolver, members Memberships) *Access {
	return &Access{resolve: resolve, members: members}
}

// Role returns the caller's active role for the workspace.
func (a *Access) Role(ctx context.Context, token, workspaceID string) (domain.Role, error) {
	actor, err := a.resolve(ctx, token)
	switch {
	case errors.Is(err, identity.ErrInvalidSession), errors.Is(err, identity.ErrInactiveAccount):
		return "", err
	case err != nil:
		return "", fmt.Errorf("%w: %w", ErrIdentityUnavailable, err)
	}
	role, err := a.members.ActiveRole(ctx, workspaceID, actor.ID)
	if err != nil && !errors.Is(err, domain.ErrMembershipNotFound) {
		return "", fmt.Errorf("%w: %w", ErrWorkspaceUnavailable, err)
	}
	return role, err
}
