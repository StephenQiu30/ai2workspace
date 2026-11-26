// Package postgres implements production workspace storage.
package postgres

import (
	"context"
	"errors"

	"github.com/StephenQiu30/ai2workspace/backend/internal/creation/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Memberships stores workspace membership in PostgreSQL.
type Memberships struct {
	pool *pgxpool.Pool
}

// NewMemberships builds workspace membership storage on the shared pool.
func NewMemberships(pool *pgxpool.Pool) *Memberships {
	return &Memberships{pool: pool}
}

// ActiveRole reads the current role for a workspace member.
func (m *Memberships) ActiveRole(ctx context.Context, workspaceID, userID string) (domain.Role, error) {
	var role domain.Role
	err := m.pool.QueryRow(ctx, `SELECT role FROM workspace_members
		WHERE workspace_id = $1 AND user_id = $2 AND revoked_at IS NULL`, workspaceID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrMembershipNotFound
	}
	return role, err
}
