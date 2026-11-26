package access

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	accessapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/access"
	accessdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/access"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store backed by PostgreSQL via pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a PostgreSQL-backed access store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

var _ accessapp.Store = (*PostgresStore)(nil)

func scanInvitation(row pgx.Row) (*accessdomain.Invitation, error) {
	var inv accessdomain.Invitation
	err := row.Scan(
		&inv.ID,
		&inv.Email,
		&inv.InvitedBy,
		&inv.Status,
		&inv.TokenHash,
		&inv.Version,
		&inv.ExpiresAt,
		&inv.CreatedAt,
		&inv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// CreateInvitation inserts a new pending invitation with a hashed single-use token.
func (s *PostgresStore) CreateInvitation(ctx context.Context, email, invitedBy string, ttl time.Duration) (*accessdomain.Invitation, string, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	rawToken := accessapp.GenerateRandomToken(16)
	tHash := accessapp.HashToken(rawToken)
	now := time.Now().UTC()

	inv := &accessdomain.Invitation{
		ID:        accessapp.NewID("inv"),
		Email:     normEmail,
		InvitedBy: invitedBy,
		Status:    "pending",
		TokenHash: tHash,
		Version:   1,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO instructor_invitations (id, email, invited_by, status, token_hash, version, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, inv.ID, inv.Email, inv.InvitedBy, inv.Status, inv.TokenHash, inv.Version, inv.ExpiresAt, inv.CreatedAt, inv.UpdatedAt)
	if err != nil {
		return nil, "", err
	}
	invCopy := *inv
	return &invCopy, rawToken, nil
}

// GetInvitation fetches an invitation by ID.
func (s *PostgresStore) GetInvitation(ctx context.Context, id string) (*accessdomain.Invitation, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, invited_by, status, token_hash, version, expires_at, created_at, updated_at
		FROM instructor_invitations
		WHERE id = $1
	`, id)
	inv, err := scanInvitation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accessdomain.ErrInvitationNotFound
		}
		return nil, err
	}
	return inv, nil
}

// GetInvitationByToken looks up an invitation by raw token hash.
func (s *PostgresStore) GetInvitationByToken(ctx context.Context, rawToken string) (*accessdomain.Invitation, error) {
	tHash := accessapp.HashToken(rawToken)
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, invited_by, status, token_hash, version, expires_at, created_at, updated_at
		FROM instructor_invitations
		WHERE token_hash = $1
	`, tHash)
	inv, err := scanInvitation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accessdomain.ErrInvitationNotFound
		}
		return nil, err
	}
	return inv, nil
}

// ListInvitations returns invitations ordered by created_at DESC with optional status filter.
func (s *PostgresStore) ListInvitations(ctx context.Context, filter accessdomain.ListInvitationsFilter) ([]*accessdomain.Invitation, int, error) {
	page, pageSize := accessapp.NormalizePagination(filter.Page, filter.PageSize)
	statusFilter := strings.TrimSpace(filter.Status)

	var whereSQL string
	var args []any
	argIdx := 1
	if statusFilter != "" {
		switch statusFilter {
		case "expired":
			whereSQL = " WHERE (status = 'expired' OR (status = 'pending' AND expires_at < NOW()))"
		case "pending":
			whereSQL = " WHERE (status = 'pending' AND expires_at >= NOW())"
		default:
			whereSQL = fmt.Sprintf(" WHERE status = $%d", argIdx)
			args = append(args, statusFilter)
			argIdx++
		}
	}

	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM instructor_invitations"+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count invitations: %w", err)
	}

	offset := (page - 1) * pageSize
	listSQL := fmt.Sprintf(`
		SELECT id, email, invited_by, status, token_hash, version, expires_at, created_at, updated_at
		FROM instructor_invitations%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)
	listArgs := append(append([]any(nil), args...), pageSize, offset)

	rows, err := s.pool.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	items := make([]*accessdomain.Invitation, 0)
	for rows.Next() {
		inv, err := scanInvitation(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan invitation: %w", err)
		}
		if inv.Status == "pending" && now.After(inv.ExpiresAt) {
			inv.Status = "expired"
		}
		items = append(items, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate invitations: %w", err)
	}
	return items, total, nil
}

// AcceptInvitation atomically validates and accepts a pending invitation.
func (s *PostgresStore) AcceptInvitation(ctx context.Context, rawToken string) (*accessdomain.Invitation, error) {
	tHash := accessapp.HashToken(rawToken)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		SELECT id, email, invited_by, status, token_hash, version, expires_at, created_at, updated_at
		FROM instructor_invitations
		WHERE token_hash = $1
		FOR UPDATE
	`, tHash)
	inv, err := scanInvitation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accessdomain.ErrInvitationNotFound
		}
		return nil, err
	}
	if inv.Status != "pending" {
		return nil, accessdomain.ErrInvitationNotPending
	}
	if time.Now().After(inv.ExpiresAt) {
		_, _ = tx.Exec(ctx, `UPDATE instructor_invitations SET status = 'expired', updated_at = NOW() WHERE id = $1`, inv.ID)
		_ = tx.Commit(ctx)
		return nil, accessdomain.ErrInvitationExpired
	}

	inv.Status = "accepted"
	inv.Version++
	inv.UpdatedAt = time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE instructor_invitations
		SET status = $1, version = $2, updated_at = $3
		WHERE id = $4
	`, inv.Status, inv.Version, inv.UpdatedAt, inv.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return inv, nil
}

// CancelInvitation cancels a pending invitation with optimistic version check.
func (s *PostgresStore) CancelInvitation(ctx context.Context, id string, expectedVersion int) (*accessdomain.Invitation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		SELECT id, email, invited_by, status, token_hash, version, expires_at, created_at, updated_at
		FROM instructor_invitations
		WHERE id = $1
		FOR UPDATE
	`, id)
	inv, err := scanInvitation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accessdomain.ErrInvitationNotFound
		}
		return nil, err
	}
	if inv.Version != expectedVersion {
		return nil, accessdomain.ErrVersionConflict
	}
	if inv.Status != "pending" {
		return nil, accessdomain.ErrInvitationNotPending
	}

	inv.Status = "cancelled"
	inv.Version++
	inv.UpdatedAt = time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE instructor_invitations
		SET status = $1, version = $2, updated_at = $3
		WHERE id = $4
	`, inv.Status, inv.Version, inv.UpdatedAt, inv.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return inv, nil
}

// RecordAuditLog inserts an audit log row.
func (s *PostgresStore) RecordAuditLog(ctx context.Context, entry accessdomain.AuditLog) error {
	if entry.ID == "" {
		entry.ID = accessapp.NewID("aud")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_logs (id, actor_id, target_id, action, detail, reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, entry.ID, entry.ActorID, entry.TargetID, entry.Action, entry.Detail, entry.Reason, entry.CreatedAt)
	return err
}

// ListAuditLogs fetches audit logs filtered by targetID (or all if empty), newest first.
func (s *PostgresStore) ListAuditLogs(ctx context.Context, targetID string) ([]accessdomain.AuditLog, error) {
	var rows pgx.Rows
	var err error
	if targetID == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, actor_id, target_id, action, detail, reason, created_at
			FROM audit_logs
			ORDER BY created_at DESC, id DESC
			LIMIT 200
		`)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, actor_id, target_id, action, detail, reason, created_at
			FROM audit_logs
			WHERE target_id = $1
			ORDER BY created_at DESC, id DESC
			LIMIT 200
		`, targetID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]accessdomain.AuditLog, 0)
	for rows.Next() {
		var entry accessdomain.AuditLog
		if err := rows.Scan(&entry.ID, &entry.ActorID, &entry.TargetID, &entry.Action, &entry.Detail, &entry.Reason, &entry.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}
