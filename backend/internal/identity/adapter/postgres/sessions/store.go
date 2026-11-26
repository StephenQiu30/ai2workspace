package sessions

import (
	"context"
	"errors"
	"time"

	sessionsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/sessions"
	sessionsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/sessions"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store backed by PostgreSQL via pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a PostgreSQL-backed session store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

var _ sessionsapp.Store = (*PostgresStore)(nil)

// CreateSession generates a random opaque session token, stores its SHA-256 hash, and returns the raw token.
func (s *PostgresStore) CreateSession(ctx context.Context, userID string, ttl time.Duration) (string, *sessionsdomain.Session, error) {
	rawToken := sessionsapp.GenerateRandomToken(32)
	sID := sessionsapp.HashToken(rawToken)
	now := time.Now().UTC()
	sess := &sessionsdomain.Session{
		ID:        sID,
		UserID:    userID,
		ExpiresAt: now.Add(ttl),
		Revoked:   false,
		Version:   1,
		CreatedAt: now,
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, expires_at, revoked, version, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, sess.ID, sess.UserID, sess.ExpiresAt, sess.Revoked, sess.Version, sess.CreatedAt)
	if err != nil {
		return "", nil, err
	}
	sessCopy := *sess
	return rawToken, &sessCopy, nil
}

// GetSession retrieves an active session by raw token, rejecting revoked or expired sessions.
func (s *PostgresStore) GetSession(ctx context.Context, rawToken string) (*sessionsdomain.Session, error) {
	sID := sessionsapp.HashToken(rawToken)
	var sess sessionsdomain.Session
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked, version, created_at
		FROM sessions
		WHERE id = $1
	`, sID).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.Revoked, &sess.Version, &sess.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sessionsdomain.ErrSessionNotFound
		}
		return nil, err
	}
	if sess.Revoked {
		return nil, sessionsdomain.ErrSessionRevoked
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, sessionsdomain.ErrSessionExpired
	}
	return &sess, nil
}

// GetUserIDBySession resolves the owning user ID for an active session token.
func (s *PostgresStore) GetUserIDBySession(ctx context.Context, rawToken string) (string, error) {
	sess, err := s.GetSession(ctx, rawToken)
	if err != nil {
		return "", err
	}
	return sess.UserID, nil
}

// RevokeSession marks a single session revoked idempotently.
func (s *PostgresStore) RevokeSession(ctx context.Context, rawToken string) error {
	sID := sessionsapp.HashToken(rawToken)
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET revoked = TRUE, version = version + 1
		WHERE id = $1 AND revoked = FALSE
	`, sID)
	return err
}

// RevokeAllForUser revokes all active sessions belonging to the user.
func (s *PostgresStore) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET revoked = TRUE, version = version + 1
		WHERE user_id = $1 AND revoked = FALSE
	`, userID)
	return err
}
