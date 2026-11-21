package progress

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore implements Store backed by PostgreSQL using pgxpool.
type PGStore struct {
	pool *pgxpool.Pool
}

// NewPGStore initializes a PostgreSQL-backed progress Store.
func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

func (s *PGStore) SaveProgress(ctx context.Context, userID, courseID, lessonID string, contentVersion, positionSeconds, expectedVersion int) (*Progress, error) {
	if positionSeconds < 0 {
		return nil, ErrInvalidPosition
	}
	if expectedVersion < 0 {
		return nil, ErrVersionConflict
	}

	now := time.Now().UTC()
	if expectedVersion == 0 {
		p := &Progress{
			ID:              generateProgressID(),
			UserID:          userID,
			CourseID:        courseID,
			LessonID:        lessonID,
			ContentVersion:  contentVersion,
			PositionSeconds: positionSeconds,
			Version:         1,
			UpdatedAt:       now,
		}
		_, err := s.pool.Exec(ctx, `
			INSERT INTO learning_progress (id, user_id, lesson_id, course_id, content_version, position_seconds, version, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 1, $7)
		`, p.ID, p.UserID, p.LessonID, p.CourseID, p.ContentVersion, p.PositionSeconds, p.UpdatedAt)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return nil, ErrVersionConflict
			}
			return nil, err
		}
		return p, nil
	}

	var p Progress
	err := s.pool.QueryRow(ctx, `
		UPDATE learning_progress
		SET course_id = $1, content_version = $2, position_seconds = $3, version = version + 1, updated_at = $4
		WHERE user_id = $5 AND lesson_id = $6 AND version = $7
		RETURNING id, user_id, course_id, lesson_id, content_version, position_seconds, version, updated_at
	`, courseID, contentVersion, positionSeconds, now, userID, lessonID, expectedVersion).
		Scan(&p.ID, &p.UserID, &p.CourseID, &p.LessonID, &p.ContentVersion, &p.PositionSeconds, &p.Version, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVersionConflict
		}
		return nil, err
	}
	return &p, nil
}

func (s *PGStore) GetProgress(ctx context.Context, userID, lessonID string) (*Progress, error) {
	var p Progress
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, course_id, lesson_id, content_version, position_seconds, version, updated_at
		FROM learning_progress
		WHERE user_id = $1 AND lesson_id = $2
	`, userID, lessonID).
		Scan(&p.ID, &p.UserID, &p.CourseID, &p.LessonID, &p.ContentVersion, &p.PositionSeconds, &p.Version, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProgressNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *PGStore) ListProgressByCourse(ctx context.Context, userID, courseID string) ([]*Progress, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, course_id, lesson_id, content_version, position_seconds, version, updated_at
		FROM learning_progress
		WHERE user_id = $1 AND course_id = $2
		ORDER BY updated_at DESC, id DESC
	`, userID, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*Progress, 0)
	for rows.Next() {
		var p Progress
		if err := rows.Scan(&p.ID, &p.UserID, &p.CourseID, &p.LessonID, &p.ContentVersion, &p.PositionSeconds, &p.Version, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &p)
	}
	return list, rows.Err()
}
