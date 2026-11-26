package media

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	mediadomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/media"
)

// Store defines media persistence operations.
type Store interface {
	CreateUpload(ctx context.Context, instructorID, courseID, lessonID, fileName string, declaredBytes int64) (*mediadomain.UploadSession, error)
	GetUpload(ctx context.Context, uploadID string) (*mediadomain.UploadSession, error)
	ListActiveUploads(ctx context.Context, instructorID string) ([]*mediadomain.UploadSession, error)
	CancelUpload(ctx context.Context, uploadID, instructorID string, isAdmin bool) error
	CompleteUpload(ctx context.Context, uploadID, instructorID string, isAdmin bool, idempotencyKey string, meta mediadomain.MediaMetadata) (*mediadomain.MediaVersion, error)
	GetMediaVersion(ctx context.Context, versionID string) (*mediadomain.MediaVersion, error)
	PublishLessonMedia(ctx context.Context, lessonID, mediaVersionID, instructorID string, isAdmin bool) error
	RetryMediaJob(ctx context.Context, jobID, instructorID string, isAdmin bool) (*mediadomain.MediaJob, error)
	GetLessonMediaVersion(ctx context.Context, lessonID string) (*mediadomain.MediaVersion, error)
}

type idempotencyRecord struct {
	payloadHash    string
	mediaVersionID string
}

// MemoryStore provides a thread-safe in-memory store for media operations.
type MemoryStore struct {
	mu            sync.RWMutex
	uploads       map[string]*mediadomain.UploadSession
	mediaVersions map[string]*mediadomain.MediaVersion
	jobs          map[string]*mediadomain.MediaJob
	lessonMedia   map[string]string            // lessonID -> published mediaVersionID
	completedKeys map[string]idempotencyRecord // uploadID + ":" + idempotencyKey -> record
}

// NewMemoryStore initializes a memory store for media.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		uploads:       make(map[string]*mediadomain.UploadSession),
		mediaVersions: make(map[string]*mediadomain.MediaVersion),
		jobs:          make(map[string]*mediadomain.MediaJob),
		lessonMedia:   make(map[string]string),
		completedKeys: make(map[string]idempotencyRecord),
	}
}

func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func HashMetadata(meta mediadomain.MediaMetadata) string {
	normalized := mediadomain.MediaMetadata{
		Container:       strings.ToLower(strings.TrimSpace(meta.Container)),
		Codec:           strings.ToLower(strings.TrimSpace(meta.Codec)),
		DurationSeconds: meta.DurationSeconds,
		Width:           meta.Width,
		Height:          meta.Height,
		FPS:             meta.FPS,
		ActualBytes:     meta.ActualBytes,
	}
	raw, _ := json.Marshal(normalized)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func cloneUpload(u *mediadomain.UploadSession) *mediadomain.UploadSession {
	if u == nil {
		return nil
	}
	cp := *u
	if u.UploadInstructions.Headers != nil {
		headers := make(map[string]string, len(u.UploadInstructions.Headers))
		for k, v := range u.UploadInstructions.Headers {
			headers[k] = v
		}
		cp.UploadInstructions.Headers = headers
	}
	return &cp
}

func cloneVersion(v *mediadomain.MediaVersion) *mediadomain.MediaVersion {
	if v == nil {
		return nil
	}
	cp := *v
	if v.HLSQualities != nil {
		cp.HLSQualities = append([]string(nil), v.HLSQualities...)
	}
	return &cp
}

func cloneJob(j *mediadomain.MediaJob) *mediadomain.MediaJob {
	if j == nil {
		return nil
	}
	cp := *j
	return &cp
}

func (s *MemoryStore) CreateUpload(ctx context.Context, instructorID, courseID, lessonID, fileName string, declaredBytes int64) (*mediadomain.UploadSession, error) {
	if declaredBytes <= 0 {
		return nil, mediadomain.ErrFileEmpty
	}
	if declaredBytes > mediadomain.MaxDeclaredBytes {
		return nil, mediadomain.ErrFileTooLarge
	}
	if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(fileName)), ".mp4") {
		return nil, mediadomain.ErrInvalidContainer
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	activeCount := 0
	for _, u := range s.uploads {
		if u.InstructorID == instructorID && u.Status == mediadomain.StatusUploading && u.ExpiresAt.After(now) {
			activeCount++
		}
	}
	if activeCount >= mediadomain.MaxConcurrentUploads {
		return nil, mediadomain.ErrMaxConcurrentUploads
	}

	uploadID := NewID("upl")
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

	s.uploads[uploadID] = session
	return cloneUpload(session), nil
}

func (s *MemoryStore) GetUpload(ctx context.Context, uploadID string) (*mediadomain.UploadSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.uploads[uploadID]
	if !exists {
		return nil, mediadomain.ErrUploadNotFound
	}
	return cloneUpload(session), nil
}

func (s *MemoryStore) ListActiveUploads(ctx context.Context, instructorID string) ([]*mediadomain.UploadSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()
	var list []*mediadomain.UploadSession
	for _, u := range s.uploads {
		if u.InstructorID == instructorID && u.Status == mediadomain.StatusUploading && u.ExpiresAt.After(now) {
			list = append(list, cloneUpload(u))
		}
	}
	return list, nil
}

func (s *MemoryStore) CancelUpload(ctx context.Context, uploadID, instructorID string, isAdmin bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.uploads[uploadID]
	if !exists {
		return mediadomain.ErrUploadNotFound
	}
	if !isAdmin && (instructorID == "" || session.InstructorID != instructorID) {
		return mediadomain.ErrPermissionDenied
	}
	if session.Status == mediadomain.StatusCompleted {
		return mediadomain.ErrUploadCompleted
	}
	session.Status = mediadomain.StatusCancelled
	session.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *MemoryStore) CompleteUpload(ctx context.Context, uploadID, instructorID string, isAdmin bool, idempotencyKey string, meta mediadomain.MediaMetadata) (*mediadomain.MediaVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.uploads[uploadID]
	if !exists {
		return nil, mediadomain.ErrUploadNotFound
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

	payloadHash := HashMetadata(meta)
	cacheKey := uploadID + ":" + idempotencyKey
	if existing, ok := s.completedKeys[cacheKey]; ok {
		if existing.payloadHash != payloadHash {
			return nil, mediadomain.ErrIdempotencyConflict
		}
		if v, ok := s.mediaVersions[existing.mediaVersionID]; ok {
			if v.Status == mediadomain.MediaFailed {
				return cloneVersion(v), errors.New(v.FailureReason)
			}
			return cloneVersion(v), nil
		}
	}

	validationErr := mediadomain.ValidateMedia(meta)

	mediaVersionID := NewID("mv")
	jobID := NewID("job")

	version := &mediadomain.MediaVersion{
		ID:              mediaVersionID,
		UploadID:        uploadID,
		LessonID:        session.LessonID,
		DurationSeconds: meta.DurationSeconds,
		Width:           meta.Width,
		Height:          meta.Height,
		FPS:             meta.FPS,
		Format:          "hls",
		CreatedAt:       now,
	}

	job := &mediadomain.MediaJob{
		ID:             jobID,
		UploadID:       uploadID,
		MediaVersionID: mediaVersionID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if validationErr != nil {
		version.Status = mediadomain.MediaFailed
		version.FailureReason = validationErr.Error()
		job.Status = "failed"
		job.ErrorMessage = validationErr.Error()
		session.Status = mediadomain.StatusFailed
	} else {
		version.Status = mediadomain.MediaReady
		version.PlaybackURL = fmt.Sprintf("/media/hls/%s/master.m3u8", mediaVersionID)
		version.HLSQualities = mediadomain.DetermineQualities(meta.Width, meta.Height)
		job.Status = "completed"
		session.Status = mediadomain.StatusCompleted
	}

	session.ActualBytes = meta.ActualBytes
	session.MediaVersionID = mediaVersionID
	session.UpdatedAt = now

	s.mediaVersions[mediaVersionID] = version
	s.jobs[jobID] = job
	s.completedKeys[cacheKey] = idempotencyRecord{
		payloadHash:    payloadHash,
		mediaVersionID: mediaVersionID,
	}

	if validationErr != nil {
		return cloneVersion(version), validationErr
	}
	return cloneVersion(version), nil
}

func (s *MemoryStore) GetMediaVersion(ctx context.Context, versionID string) (*mediadomain.MediaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, exists := s.mediaVersions[versionID]
	if !exists {
		return nil, mediadomain.ErrMediaVersionNotFound
	}
	return cloneVersion(v), nil
}

func (s *MemoryStore) PublishLessonMedia(ctx context.Context, lessonID, mediaVersionID, instructorID string, isAdmin bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, exists := s.mediaVersions[mediaVersionID]
	if !exists {
		return mediadomain.ErrMediaVersionNotFound
	}
	if v.LessonID != lessonID {
		return mediadomain.ErrLessonMismatch
	}
	if v.Status != mediadomain.MediaReady {
		return mediadomain.ErrMediaNotReady
	}

	if session, exists := s.uploads[v.UploadID]; exists {
		if !isAdmin && (instructorID == "" || session.InstructorID != instructorID) {
			return mediadomain.ErrPermissionDenied
		}
	}

	s.lessonMedia[lessonID] = mediaVersionID
	return nil
}

func (s *MemoryStore) GetLessonMediaVersion(ctx context.Context, lessonID string) (*mediadomain.MediaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versionID, exists := s.lessonMedia[lessonID]
	if !exists {
		return nil, mediadomain.ErrMediaVersionNotFound
	}
	v, exists := s.mediaVersions[versionID]
	if !exists {
		return nil, mediadomain.ErrMediaVersionNotFound
	}
	return cloneVersion(v), nil
}

func (s *MemoryStore) RetryMediaJob(ctx context.Context, jobID, instructorID string, isAdmin bool) (*mediadomain.MediaJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return nil, mediadomain.ErrMediaJobNotFound
	}
	if session, ok := s.uploads[job.UploadID]; ok {
		if !isAdmin && (instructorID == "" || session.InstructorID != instructorID) {
			return nil, mediadomain.ErrPermissionDenied
		}
	} else if !isAdmin {
		return nil, mediadomain.ErrPermissionDenied
	}
	if job.Status != "failed" {
		return nil, mediadomain.ErrJobNotRetryable
	}

	job.Status = "pending"
	job.RetryCount++
	job.ErrorMessage = ""
	job.UpdatedAt = time.Now().UTC()

	if v, exists := s.mediaVersions[job.MediaVersionID]; exists {
		v.Status = mediadomain.MediaProcessing
		v.FailureReason = ""
	}

	return cloneJob(job), nil
}
