package entitlements

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store backed by PostgreSQL via pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a PostgreSQL-backed entitlements store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) GrantEntitlement(ctx context.Context, userID, courseID, orderID string) (*Entitlement, error) {
	now := time.Now().UTC()
	entID := generateID("ent")
	var e Entitlement
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO entitlements (id, user_id, course_id, order_id, status, grant_version, activated_at)
		VALUES ($1, $2, $3, $4, $5, 1, $6)
		ON CONFLICT (order_id) DO NOTHING
		RETURNING id, user_id, course_id, order_id, status, grant_version, activated_at, revoked_at
	`, entID, userID, courseID, orderID, string(GrantActive), now).Scan(
		&e.ID, &e.UserID, &e.CourseID, &e.OrderID, &statusStr, &e.GrantVersion, &e.ActivatedAt, &e.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `
			SELECT id, user_id, course_id, order_id, status, grant_version, activated_at, revoked_at
			FROM entitlements WHERE order_id = $1
		`, orderID).Scan(
			&e.ID, &e.UserID, &e.CourseID, &e.OrderID, &statusStr, &e.GrantVersion, &e.ActivatedAt, &e.RevokedAt,
		)
	}
	if err != nil {
		return nil, err
	}
	e.Status = GrantStatus(statusStr)
	return &e, nil
}

func (s *PostgresStore) CheckEntitlement(ctx context.Context, userID, courseID, _ string) (bool, int, string, error) {
	var grantVersion int
	err := s.pool.QueryRow(ctx, `
		SELECT grant_version
		FROM entitlements
		WHERE user_id = $1 AND course_id = $2 AND status = $3
		ORDER BY activated_at DESC, id DESC
		LIMIT 1
	`, userID, courseID, string(GrantActive)).Scan(&grantVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, 0, "not_purchased", nil
	}
	if err != nil {
		return false, 0, "", err
	}
	return true, grantVersion, "active_purchase_grant", nil
}

func (s *PostgresStore) RevokeEntitlement(ctx context.Context, orderID string) error {
	now := time.Now().UTC()
	tag, err := s.pool.Exec(ctx, `
		UPDATE entitlements
		SET status = $2, revoked_at = $3, grant_version = grant_version + 1
		WHERE order_id = $1 AND status = $4
	`, orderID, string(GrantRevoked), now, string(GrantActive))
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	var statusStr string
	err = s.pool.QueryRow(ctx, `SELECT status FROM entitlements WHERE order_id = $1`, orderID).Scan(&statusStr)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEntitlementNotFound
	}
	if err != nil {
		return err
	}
	// Already revoked -> idempotent success.
	return nil
}

func (s *PostgresStore) ListUserEntitlements(ctx context.Context, userID string) ([]*Entitlement, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, course_id, order_id, status, grant_version, activated_at, revoked_at
		FROM entitlements
		WHERE user_id = $1
		ORDER BY activated_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*Entitlement, 0)
	for rows.Next() {
		var e Entitlement
		var statusStr string
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.CourseID, &e.OrderID, &statusStr, &e.GrantVersion, &e.ActivatedAt, &e.RevokedAt,
		); err != nil {
			return nil, err
		}
		e.Status = GrantStatus(statusStr)
		list = append(list, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
