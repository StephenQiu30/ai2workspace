package interaction

import (
	"context"
	"fmt"
	"time"

	interactionapp "github.com/StephenQiu30/ai2workspace/backend/internal/community/application/interaction"
	interactiondomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/interaction"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists reactions and favorites in PostgreSQL using pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a PostgreSQL-backed interaction store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// SetReaction atomically updates the user's reaction state and returns recalculated counts.
func (s *PostgresStore) SetReaction(ctx context.Context, userID, targetType, targetID string, value interactiondomain.ReactionValue) (*interactiondomain.ReactionSummary, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, interactiondomain.ErrInvalidTargetType
	}
	if !interactiondomain.IsValidReactionValue(value) {
		return nil, interactiondomain.ErrInvalidReactionValue
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin reaction tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	if value == interactiondomain.ReactionNone {
		if _, err := tx.Exec(ctx, `
			DELETE FROM reactions
			WHERE user_id = $1 AND target_type = $2 AND target_id = $3
		`, userID, targetType, targetID); err != nil {
			return nil, fmt.Errorf("delete reaction: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, `
			INSERT INTO reactions (id, user_id, target_type, target_id, value, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6)
			ON CONFLICT (user_id, target_type, target_id)
			DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at
		`, interactionapp.NewID("rx"), userID, targetType, targetID, string(value), now); err != nil {
			return nil, fmt.Errorf("upsert reaction: %w", err)
		}
	}

	var likes, dislikes int
	if err := tx.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN value = 'like' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN value = 'dislike' THEN 1 ELSE 0 END), 0)
		FROM reactions
		WHERE target_type = $1 AND target_id = $2
	`, targetType, targetID).Scan(&likes, &dislikes); err != nil {
		return nil, fmt.Errorf("count reactions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit reaction tx: %w", err)
	}

	return &interactiondomain.ReactionSummary{
		TargetType: targetType,
		TargetID:   targetID,
		Likes:      likes,
		Dislikes:   dislikes,
		MyReaction: value,
	}, nil
}

// GetReaction returns counts and the viewer's current reaction.
func (s *PostgresStore) GetReaction(ctx context.Context, userID, targetType, targetID string) (*interactiondomain.ReactionSummary, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, interactiondomain.ErrInvalidTargetType
	}

	var likes, dislikes int
	if err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN value = 'like' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN value = 'dislike' THEN 1 ELSE 0 END), 0)
		FROM reactions
		WHERE target_type = $1 AND target_id = $2
	`, targetType, targetID).Scan(&likes, &dislikes); err != nil {
		return nil, fmt.Errorf("count reactions: %w", err)
	}

	myReaction := interactiondomain.ReactionNone
	if userID != "" {
		var val string
		err := s.pool.QueryRow(ctx, `
			SELECT value FROM reactions
			WHERE user_id = $1 AND target_type = $2 AND target_id = $3
		`, userID, targetType, targetID).Scan(&val)
		if err == nil {
			myReaction = interactiondomain.ReactionValue(val)
		}
	}

	return &interactiondomain.ReactionSummary{
		TargetType: targetType,
		TargetID:   targetID,
		Likes:      likes,
		Dislikes:   dislikes,
		MyReaction: myReaction,
	}, nil
}

// SetFavorite bookmarks or unbookmarks a post for userID.
func (s *PostgresStore) SetFavorite(ctx context.Context, userID, postID string, saved bool) (bool, error) {
	if saved {
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO favorites (id, user_id, post_id, created_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, post_id) DO NOTHING
		`, interactionapp.NewID("fav"), userID, postID, time.Now().UTC()); err != nil {
			return false, fmt.Errorf("insert favorite: %w", err)
		}
		return true, nil
	}

	if _, err := s.pool.Exec(ctx, `
		DELETE FROM favorites WHERE user_id = $1 AND post_id = $2
	`, userID, postID); err != nil {
		return false, fmt.Errorf("delete favorite: %w", err)
	}
	return false, nil
}

// IsFavorited returns true when userID has bookmarked postID.
func (s *PostgresStore) IsFavorited(ctx context.Context, userID, postID string) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM favorites WHERE user_id = $1 AND post_id = $2)
	`, userID, postID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check favorite: %w", err)
	}
	return exists, nil
}

// ListUserFavorites returns bookmarked post IDs ordered by created_at DESC, post_id DESC.
func (s *PostgresStore) ListUserFavorites(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT post_id
		FROM favorites
		WHERE user_id = $1
		ORDER BY created_at DESC, post_id DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	defer rows.Close()

	postIDs := make([]string, 0)
	for rows.Next() {
		var postID string
		if err := rows.Scan(&postID); err != nil {
			return nil, fmt.Errorf("scan favorite: %w", err)
		}
		postIDs = append(postIDs, postID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate favorites: %w", err)
	}
	return postIDs, nil
}
