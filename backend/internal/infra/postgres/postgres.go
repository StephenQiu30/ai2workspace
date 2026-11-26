package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open connects to the single initialized PostgreSQL database.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		// A parse error can contain credentials from the connection string.
		return nil, errors.New("invalid DATABASE_URL")
	}

	if _, err := pool.Exec(ctx, "SELECT 1 FROM workspace_members LIMIT 0"); err != nil {
		pool.Close()
		return nil, fmt.Errorf("open initialized database (apply backend/schema.sql to an empty database): %w", err)
	}
	return pool, nil
}
