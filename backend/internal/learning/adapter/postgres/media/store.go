package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mediaapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/media"
	mediadomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/media"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store backed by PostgreSQL using pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore initializes a PostgreSQL-backed media Store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) CreateUpload(ctx context.Context, instructorID, courseID, lessonID, fileName string, declaredBytes int64) (*mediadomain.UploadSession, error) {
	if declaredBytes <= 0 {
		return nil, mediadomain.ErrFileEmpty
	}
	if declaredBytes > mediadomain.MaxDeclaredBytes {
		return nil, mediadomain.ErrFileTooLarge
	}
	if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(fileName)), ".mp4") {
		return nil, mediadomain.ErrInvalidContainer
	}

	now := time.Now().UTC()
	var activeCount int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM uploads
		WHERE instructor_id = $1 AND status = 'uploading' AND expires_at > $2
	`, instructorID, now).Scan(&activeCount); err != nil {
		return nil, err
	}
	if activeCount >= mediadomain.MaxConcurrentUploads {
		return nil, mediadomain.ErrMaxConcurrentUploads
	}

	uploadID := mediaapp.NewID("upl")
	session := &mediadomain.UploadSession{
		ID:            uploadID,
		InstructorID:  instructorID,
		CourseID:      courseID,
		LessonID:      lessonID,
		FileName:      fileName,
		DeclaredBytes: declaredBytes,
		Status:        mediadomain.StatusUploading,
		SourceVersion: 1,
		ExpiresAt:     now.Add(mediadomain.UploadExpiryDuration),
		UploadInstructions: mediadomain.UploadInstructions{
			UploadURL:    fmt.Sprintf("https://storage.learning.local/uploads/%s", uploadID),
			UploadMethod: "PUT",
			Headers: map[string]string{
				"Content-Type": "video/mp4",
				"X-Upload-ID":  uploadID,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	instrJSON, err := json.Marshal(session.UploadInstructions)
	if err != nil {
		return nil, err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO uploads (
			id, instructor_id, course_id, lesson_id, file_name,
			declared_bytes, actual_bytes, status, source_version,
			expires_at, upload_instructions, media_version_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, 0, $7, $8,
			$9, $10, '', $11, $11
		)
	`, session.ID, session.InstructorID, session.CourseID, session.LessonID, session.FileName,
		session.DeclaredBytes, string(session.Status), session.SourceVersion,
		session.ExpiresAt, instrJSON, now)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func scanUploadSession(row pgx.Row) (*mediadomain.UploadSession, error) {
	var u mediadomain.UploadSession
	var statusStr string
	var instrBytes []byte
	err := row.Scan(
		&u.ID, &u.InstructorID, &u.CourseID, &u.LessonID, &u.FileName,
		&u.DeclaredBytes, &u.ActualBytes, &statusStr, &u.SourceVersion,
		&u.ExpiresAt, &instrBytes, &u.MediaVersionID, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, mediadomain.ErrUploadNotFound
		}
		return nil, err
	}
	u.Status = mediadomain.UploadStatus(statusStr)
	if len(instrBytes) > 0 {
		_ = json.Unmarshal(instrBytes, &u.UploadInstructions)
	}
	return &u, nil
}

const uploadSelectCols = `
	id, instructor_id, course_id, lesson_id, file_name,
	declared_bytes, COALESCE(actual_bytes, 0), status, source_version,
	expires_at, COALESCE(upload_instructions, '{}'::jsonb), COALESCE(media_version_id, ''),
	created_at, updated_at
`

func (s *PostgresStore) GetUpload(ctx context.Context, uploadID string) (*mediadomain.UploadSession, error) {
	return scanUploadSession(s.pool.QueryRow(ctx, `SELECT `+uploadSelectCols+` FROM uploads WHERE id = $1`, uploadID))
}

func (s *PostgresStore) ListActiveUploads(ctx context.Context, instructorID string) ([]*mediadomain.UploadSession, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+uploadSelectCols+`
		FROM uploads
		WHERE instructor_id = $1 AND status = 'uploading' AND expires_at > $2
	`, instructorID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*mediadomain.UploadSession, 0)
	for rows.Next() {
		var u mediadomain.UploadSession
		var statusStr string
		var instrBytes []byte
		if err := rows.Scan(
			&u.ID, &u.InstructorID, &u.CourseID, &u.LessonID, &u.FileName,
			&u.DeclaredBytes, &u.ActualBytes, &statusStr, &u.SourceVersion,
			&u.ExpiresAt, &instrBytes, &u.MediaVersionID, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		u.Status = mediadomain.UploadStatus(statusStr)
		if len(instrBytes) > 0 {
			_ = json.Unmarshal(instrBytes, &u.UploadInstructions)
		}
		list = append(list, &u)
	}
	return list, rows.Err()
}

func (s *PostgresStore) CancelUpload(ctx context.Context, uploadID, instructorID string, isAdmin bool) error {
	session, err := s.GetUpload(ctx, uploadID)
	if err != nil {
		return err
	}
	if !isAdmin && (instructorID == "" || session.InstructorID != instructorID) {
		return mediadomain.ErrPermissionDenied
	}
	if session.Status == mediadomain.StatusCompleted {
		return mediadomain.ErrUploadCompleted
	}
	_, err = s.pool.Exec(ctx, `UPDATE uploads SET status = 'cancelled', updated_at = $1 WHERE id = $2`, time.Now().UTC(), uploadID)
	return err
}

func (s *PostgresStore) CompleteUpload(ctx context.Context, uploadID, instructorID string, isAdmin bool, idempotencyKey string, meta mediadomain.MediaMetadata) (*mediadomain.MediaVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	session, err := scanUploadSession(tx.QueryRow(ctx, `SELECT `+uploadSelectCols+` FROM uploads WHERE id = $1 FOR UPDATE`, uploadID))
	if err != nil {
		return nil, err
	}
	if !isAdmin && (instructorID == "" || session.InstructorID != instructorID) {
		return nil, mediadomain.ErrPermissionDenied
	}

	now := time.Now().UTC()
	if session.ExpiresAt.Before(now) {
		return nil, mediadomain.ErrUploadExpired
	}
	if session.Status == mediadomain.StatusCancelled {
		return nil, mediadomain.ErrUploadCancelled
	}

	payloadHash := mediaapp.HashMetadata(meta)
	var existingHash, existingVersionID string
	err = tx.QueryRow(ctx, `
		SELECT payload_hash, media_version_id
		FROM upload_idempotency
		WHERE upload_id = $1 AND idempotency_key = $2
	`, uploadID, idempotencyKey).Scan(&existingHash, &existingVersionID)
	if err == nil {
		if existingHash != payloadHash {
			return nil, mediadomain.ErrIdempotencyConflict
		}
		v, err := scanMediaVersion(tx.QueryRow(ctx, `SELECT `+mediaVersionCols+` FROM media_versions WHERE id = $1`, existingVersionID))
		if err != nil {
			return nil, err
		}
		if v.Status == mediadomain.MediaFailed {
			return v, errors.New(v.FailureReason)
		}
		return v, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	validationErr := mediadomain.ValidateMedia(meta)
	mediaVersionID := mediaapp.NewID("mv")
	jobID := mediaapp.NewID("job")

	version := &mediadomain.MediaVersion{
		ID:              mediaVersionID,
		UploadID:        uploadID,
		LessonID:        session.LessonID,
		DurationSeconds: meta.DurationSeconds,
		Width:           meta.Width,
		Height:          meta.Height,
		FPS:             meta.FPS,
		Format:          "hls",
		HLSQualities:    []string{},
		CreatedAt:       now,
	}

	job := &mediadomain.MediaJob{
		ID:             jobID,
		UploadID:       uploadID,
		MediaVersionID: mediaVersionID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	var uploadStatus mediadomain.UploadStatus
	if validationErr != nil {
		version.Status = mediadomain.MediaFailed
		version.FailureReason = validationErr.Error()
		job.Status = "failed"
		job.ErrorMessage = validationErr.Error()
		uploadStatus = mediadomain.StatusFailed
	} else {
		version.Status = mediadomain.MediaReady
		version.PlaybackURL = fmt.Sprintf("/media/hls/%s/master.m3u8", mediaVersionID)
		version.HLSQualities = mediadomain.DetermineQualities(meta.Width, meta.Height)
		job.Status = "completed"
		uploadStatus = mediadomain.StatusCompleted
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO media_versions (
			id, upload_id, lesson_id, status, duration_seconds,
			width, height, fps, format, playback_url, hls_qualities, failure_reason, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11, $12, $13
		)
	`, version.ID, version.UploadID, version.LessonID, string(version.Status), version.DurationSeconds,
		version.Width, version.Height, version.FPS, version.Format, version.PlaybackURL, version.HLSQualities, version.FailureReason, now); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO media_jobs (id, upload_id, media_version_id, status, retry_count, error_message, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 0, $5, $6, $6)
	`, job.ID, job.UploadID, job.MediaVersionID, job.Status, job.ErrorMessage, now); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE uploads
		SET status = $1, actual_bytes = $2, media_version_id = $3, updated_at = $4
		WHERE id = $5
	`, string(uploadStatus), meta.ActualBytes, version.ID, now, uploadID); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO upload_idempotency (upload_id, idempotency_key, payload_hash, media_version_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uploadID, idempotencyKey, payloadHash, version.ID, now); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if validationErr != nil {
		return version, validationErr
	}
	return version, nil
}

const mediaVersionCols = `
	id, COALESCE(upload_id, ''), lesson_id, status, duration_seconds,
	width, height, fps, format, playback_url, COALESCE(hls_qualities, '{}'), failure_reason, created_at
`

func scanMediaVersion(row pgx.Row) (*mediadomain.MediaVersion, error) {
	var v mediadomain.MediaVersion
	var statusStr string
	err := row.Scan(
		&v.ID, &v.UploadID, &v.LessonID, &statusStr, &v.DurationSeconds,
		&v.Width, &v.Height, &v.FPS, &v.Format, &v.PlaybackURL, &v.HLSQualities, &v.FailureReason, &v.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, mediadomain.ErrMediaVersionNotFound
		}
		return nil, err
	}
	v.Status = mediadomain.MediaVersionStatus(statusStr)
	return &v, nil
}

func (s *PostgresStore) GetMediaVersion(ctx context.Context, versionID string) (*mediadomain.MediaVersion, error) {
	return scanMediaVersion(s.pool.QueryRow(ctx, `SELECT `+mediaVersionCols+` FROM media_versions WHERE id = $1`, versionID))
}

func (s *PostgresStore) PublishLessonMedia(ctx context.Context, lessonID, mediaVersionID, instructorID string, isAdmin bool) error {
	v, err := s.GetMediaVersion(ctx, mediaVersionID)
	if err != nil {
		return err
	}
	if v.LessonID != lessonID {
		return mediadomain.ErrLessonMismatch
	}
	if v.Status != mediadomain.MediaReady {
		return mediadomain.ErrMediaNotReady
	}
	if v.UploadID != "" {
		if session, err := s.GetUpload(ctx, v.UploadID); err == nil {
			if !isAdmin && (instructorID == "" || session.InstructorID != instructorID) {
				return mediadomain.ErrPermissionDenied
			}
		}
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO lesson_media (lesson_id, media_version_id, updated_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (lesson_id) DO UPDATE SET media_version_id = EXCLUDED.media_version_id, updated_at = EXCLUDED.updated_at
	`, lessonID, mediaVersionID, time.Now().UTC())
	return err
}

func (s *PostgresStore) GetLessonMediaVersion(ctx context.Context, lessonID string) (*mediadomain.MediaVersion, error) {
	var versionID string
	err := s.pool.QueryRow(ctx, `SELECT media_version_id FROM lesson_media WHERE lesson_id = $1`, lessonID).Scan(&versionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, mediadomain.ErrMediaVersionNotFound
		}
		return nil, err
	}
	return s.GetMediaVersion(ctx, versionID)
}

func (s *PostgresStore) RetryMediaJob(ctx context.Context, jobID, instructorID string, isAdmin bool) (*mediadomain.MediaJob, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var job mediadomain.MediaJob
	err = tx.QueryRow(ctx, `
		SELECT id, upload_id, media_version_id, status, retry_count, error_message, created_at, updated_at
		FROM media_jobs
		WHERE id = $1
		FOR UPDATE
	`, jobID).Scan(&job.ID, &job.UploadID, &job.MediaVersionID, &job.Status, &job.RetryCount, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, mediadomain.ErrMediaJobNotFound
		}
		return nil, err
	}

	var ownerID string
	err = tx.QueryRow(ctx, `SELECT instructor_id FROM uploads WHERE id = $1`, job.UploadID).Scan(&ownerID)
	if err != nil {
		if !isAdmin {
			return nil, mediadomain.ErrPermissionDenied
		}
	} else if !isAdmin && (instructorID == "" || ownerID != instructorID) {
		return nil, mediadomain.ErrPermissionDenied
	}

	if job.Status != "failed" {
		return nil, mediadomain.ErrJobNotRetryable
	}

	now := time.Now().UTC()
	job.Status = "pending"
	job.RetryCount++
	job.ErrorMessage = ""
	job.UpdatedAt = now

	if _, err := tx.Exec(ctx, `
		UPDATE media_jobs
		SET status = 'pending', retry_count = $1, error_message = '', updated_at = $2
		WHERE id = $3
	`, job.RetryCount, now, job.ID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE media_versions
		SET status = 'processing', failure_reason = ''
		WHERE id = $1
	`, job.MediaVersionID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &job, nil
}
