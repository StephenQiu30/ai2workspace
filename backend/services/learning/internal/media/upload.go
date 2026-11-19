// Package media handles controlled media upload, validation, transcoding, and publication.
package media

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrUploadNotFound       = errors.New("upload session not found")
	ErrUploadExpired        = errors.New("upload session has expired")
	ErrUploadCancelled      = errors.New("upload session has been cancelled")
	ErrMaxConcurrentUploads = errors.New("exceeded maximum concurrent upload sessions (limit 2)")
	ErrFileTooLarge         = errors.New("file size exceeds maximum allowed 2 GiB (2,147,483,648 bytes)")
	ErrFileEmpty            = errors.New("file size must be greater than zero")
	ErrInvalidContainer     = errors.New("only MP4 container is allowed; please re-export your video")
	ErrInvalidCodec         = errors.New("only H.264 video codec is allowed; please re-export your video")
	ErrDurationExceeded     = errors.New("video duration exceeds maximum allowed 120 minutes")
	ErrDurationZero         = errors.New("video duration must be greater than zero")
	ErrResolutionExceeded   = errors.New("video resolution exceeds maximum allowed (long side <= 1920, short side <= 1080)")
	ErrFPSExceeded          = errors.New("video frame rate exceeds maximum allowed 30 fps")
	ErrMediaVersionNotFound = errors.New("media version not found")
	ErrMediaNotReady        = errors.New("media version is not ready for publication")
	ErrMediaJobNotFound     = errors.New("media processing job not found")
	ErrJobNotRetryable      = errors.New("only failed media jobs can be retried")
	ErrPermissionDenied     = errors.New("permission denied")
)

const (
	MaxDeclaredBytes     int64 = 2147483648 // 2 GiB
	MaxDurationSeconds   int   = 7200       // 120 minutes
	MaxLongSide          int   = 1920
	MaxShortSide         int   = 1080
	MaxFPS               int   = 30
	MaxConcurrentUploads       = 2
	UploadExpiryDuration       = 24 * time.Hour
)

// UploadStatus indicates the current state of an upload session.
type UploadStatus string

const (
	StatusUploading  UploadStatus = "uploading"
	StatusProcessing UploadStatus = "processing"
	StatusCompleted  UploadStatus = "completed"
	StatusFailed     UploadStatus = "failed"
	StatusCancelled  UploadStatus = "cancelled"
)

// MediaVersionStatus indicates the processing outcome of a media version.
type MediaVersionStatus string

const (
	MediaProcessing MediaVersionStatus = "processing"
	MediaReady      MediaVersionStatus = "ready"
	MediaFailed     MediaVersionStatus = "failed"
)

// UploadSession represents an authorized direct upload slot.
type UploadSession struct {
	ID                 string             `json:"id"`
	InstructorID       string             `json:"instructorId"`
	CourseID           string             `json:"courseId"`
	LessonID           string             `json:"lessonId"`
	FileName           string             `json:"fileName"`
	DeclaredBytes      int64              `json:"declaredBytes"`
	ActualBytes        int64              `json:"actualBytes"`
	Status             UploadStatus       `json:"status"`
	SourceVersion      int                `json:"sourceVersion"`
	ExpiresAt          time.Time          `json:"expiresAt"`
	UploadInstructions UploadInstructions `json:"uploadInstructions"`
	MediaVersionID     string             `json:"mediaVersionId,omitempty"`
	CreatedAt          time.Time          `json:"createdAt"`
	UpdatedAt          time.Time          `json:"updatedAt"`
}

// UploadInstructions provides authorized direct upload credentials to storage.
type UploadInstructions struct {
	UploadURL     string            `json:"uploadUrl"`
	UploadMethod  string            `json:"uploadMethod"`
	RequiredParts int               `json:"requiredParts,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
}

// MediaMetadata represents probed container and stream attributes.
type MediaMetadata struct {
	Container       string `json:"container"` // "mp4"
	Codec           string `json:"codec"`     // "h264"
	DurationSeconds int    `json:"durationSeconds"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	FPS             int    `json:"fps"`
	ActualBytes     int64  `json:"actualBytes"`
}

// MediaVersion represents a verified transcode output ready for delivery.
type MediaVersion struct {
	ID              string             `json:"id"`
	UploadID        string             `json:"uploadId"`
	LessonID        string             `json:"lessonId"`
	Status          MediaVersionStatus `json:"status"`
	DurationSeconds int                `json:"durationSeconds"`
	Width           int                `json:"width"`
	Height          int                `json:"height"`
	FPS             int                `json:"fps"`
	Format          string             `json:"format"` // "hls"
	PlaybackURL     string             `json:"playbackUrl"`
	HLSQualities    []string           `json:"hlsQualities"` // ["720p", "1080p"]
	FailureReason   string             `json:"failureReason,omitempty"`
	CreatedAt       time.Time          `json:"createdAt"`
}

// MediaJob represents an asynchronous transcoding/packaging unit of work.
type MediaJob struct {
	ID             string    `json:"id"`
	UploadID       string    `json:"uploadId"`
	MediaVersionID string    `json:"mediaVersionId"`
	Status         string    `json:"status"` // "pending", "running", "completed", "failed"
	RetryCount     int       `json:"retryCount"`
	ErrorMessage   string    `json:"errorMessage,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Store defines media persistence operations.
type Store interface {
	CreateUpload(ctx context.Context, instructorID, courseID, lessonID, fileName string, declaredBytes int64) (*UploadSession, error)
	GetUpload(ctx context.Context, uploadID string) (*UploadSession, error)
	ListActiveUploads(ctx context.Context, instructorID string) ([]*UploadSession, error)
	CancelUpload(ctx context.Context, uploadID, instructorID string) error
	CompleteUpload(ctx context.Context, uploadID, idempotencyKey string, meta MediaMetadata) (*MediaVersion, error)
	GetMediaVersion(ctx context.Context, versionID string) (*MediaVersion, error)
	PublishLessonMedia(ctx context.Context, lessonID, mediaVersionID, instructorID string) error
	RetryMediaJob(ctx context.Context, jobID, instructorID string) (*MediaJob, error)
	GetLessonMediaVersion(ctx context.Context, lessonID string) (*MediaVersion, error)
}

// MemoryStore provides a thread-safe in-memory store for media operations.
type MemoryStore struct {
	mu             sync.RWMutex
	uploads        map[string]*UploadSession
	mediaVersions  map[string]*MediaVersion
	jobs           map[string]*MediaJob
	lessonMedia    map[string]string // lessonID -> published mediaVersionID
	completedKeys  map[string]string // uploadID + ":" + idempotencyKey -> mediaVersionID
	courseVerifier func(courseID, instructorID string) (bool, error)
}

// NewMemoryStore initializes a memory store for media.
func NewMemoryStore(verifier func(courseID, instructorID string) (bool, error)) *MemoryStore {
	return &MemoryStore{
		uploads:        make(map[string]*UploadSession),
		mediaVersions:  make(map[string]*MediaVersion),
		jobs:           make(map[string]*MediaJob),
		lessonMedia:    make(map[string]string),
		completedKeys:  make(map[string]string),
		courseVerifier: verifier,
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (s *MemoryStore) CreateUpload(ctx context.Context, instructorID, courseID, lessonID, fileName string, declaredBytes int64) (*UploadSession, error) {
	if declaredBytes <= 0 {
		return nil, ErrFileEmpty
	}
	if declaredBytes > MaxDeclaredBytes {
		return nil, ErrFileTooLarge
	}
	if !strings.HasSuffix(strings.ToLower(fileName), ".mp4") {
		return nil, ErrInvalidContainer
	}

	if s.courseVerifier != nil {
		allowed, err := s.courseVerifier(courseID, instructorID)
		if err != nil || !allowed {
			return nil, ErrPermissionDenied
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check active upload sessions for this instructor
	now := time.Now()
	activeCount := 0
	for _, u := range s.uploads {
		if u.InstructorID == instructorID && u.Status == StatusUploading && u.ExpiresAt.After(now) {
			activeCount++
		}
	}
	if activeCount >= MaxConcurrentUploads {
		return nil, ErrMaxConcurrentUploads
	}

	uploadID := generateID("upl")
	session := &UploadSession{
		ID:            uploadID,
		InstructorID:  instructorID,
		CourseID:      courseID,
		LessonID:      lessonID,
		FileName:      fileName,
		DeclaredBytes: declaredBytes,
		Status:        StatusUploading,
		SourceVersion: 1,
		ExpiresAt:     now.Add(UploadExpiryDuration),
		UploadInstructions: UploadInstructions{
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
	return session, nil
}

func (s *MemoryStore) GetUpload(ctx context.Context, uploadID string) (*UploadSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.uploads[uploadID]
	if !exists {
		return nil, ErrUploadNotFound
	}
	return session, nil
}

func (s *MemoryStore) ListActiveUploads(ctx context.Context, instructorID string) ([]*UploadSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var list []*UploadSession
	for _, u := range s.uploads {
		if u.InstructorID == instructorID && u.Status == StatusUploading && u.ExpiresAt.After(now) {
			list = append(list, u)
		}
	}
	return list, nil
}

func (s *MemoryStore) CancelUpload(ctx context.Context, uploadID, instructorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.uploads[uploadID]
	if !exists {
		return ErrUploadNotFound
	}
	if session.InstructorID != instructorID {
		return ErrPermissionDenied
	}
	session.Status = StatusCancelled
	session.UpdatedAt = time.Now()
	return nil
}

func (s *MemoryStore) CompleteUpload(ctx context.Context, uploadID, idempotencyKey string, meta MediaMetadata) (*MediaVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.uploads[uploadID]
	if !exists {
		return nil, ErrUploadNotFound
	}

	now := time.Now()
	if session.ExpiresAt.Before(now) {
		return nil, ErrUploadExpired
	}
	if session.Status == StatusCancelled {
		return nil, ErrUploadCancelled
	}

	// Idempotent replay check
	cacheKey := uploadID + ":" + idempotencyKey
	if existingVersionID, ok := s.completedKeys[cacheKey]; ok {
		if v, ok := s.mediaVersions[existingVersionID]; ok {
			return v, nil
		}
	}

	// Validate actual media properties
	validationErr := ValidateMedia(meta)

	mediaVersionID := generateID("mv")
	jobID := generateID("job")

	version := &MediaVersion{
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

	job := &MediaJob{
		ID:             jobID,
		UploadID:       uploadID,
		MediaVersionID: mediaVersionID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if validationErr != nil {
		version.Status = MediaFailed
		version.FailureReason = validationErr.Error()
		job.Status = "failed"
		job.ErrorMessage = validationErr.Error()
		session.Status = StatusFailed
	} else {
		version.Status = MediaReady
		version.PlaybackURL = fmt.Sprintf("/media/hls/%s/master.m3u8", mediaVersionID)
		version.HLSQualities = DetermineQualities(meta.Width, meta.Height)
		job.Status = "completed"
		session.Status = StatusCompleted
	}

	session.ActualBytes = meta.ActualBytes
	session.MediaVersionID = mediaVersionID
	session.UpdatedAt = now

	s.mediaVersions[mediaVersionID] = version
	s.jobs[jobID] = job
	s.completedKeys[cacheKey] = mediaVersionID

	if validationErr != nil {
		return version, validationErr
	}
	return version, nil
}

func (s *MemoryStore) GetMediaVersion(ctx context.Context, versionID string) (*MediaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, exists := s.mediaVersions[versionID]
	if !exists {
		return nil, ErrMediaVersionNotFound
	}
	return v, nil
}

func (s *MemoryStore) PublishLessonMedia(ctx context.Context, lessonID, mediaVersionID, instructorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, exists := s.mediaVersions[mediaVersionID]
	if !exists {
		return ErrMediaVersionNotFound
	}
	if v.Status != MediaReady {
		return ErrMediaNotReady
	}

	// Verify upload ownership if upload exists
	if session, exists := s.uploads[v.UploadID]; exists {
		if session.InstructorID != instructorID {
			return ErrPermissionDenied
		}
	}

	// Safely update lesson published media version.
	// If a failed version was submitted previously, the existing published version is untouched.
	s.lessonMedia[lessonID] = mediaVersionID
	return nil
}

func (s *MemoryStore) GetLessonMediaVersion(ctx context.Context, lessonID string) (*MediaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versionID, exists := s.lessonMedia[lessonID]
	if !exists {
		return nil, ErrMediaVersionNotFound
	}
	v, exists := s.mediaVersions[versionID]
	if !exists {
		return nil, ErrMediaVersionNotFound
	}
	return v, nil
}

func (s *MemoryStore) RetryMediaJob(ctx context.Context, jobID, instructorID string) (*MediaJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return nil, ErrMediaJobNotFound
	}
	if job.Status != "failed" {
		return nil, ErrJobNotRetryable
	}

	job.Status = "pending"
	job.RetryCount++
	job.ErrorMessage = ""
	job.UpdatedAt = time.Now()

	// Update corresponding media version to processing
	if v, exists := s.mediaVersions[job.MediaVersionID]; exists {
		v.Status = MediaProcessing
		v.FailureReason = ""
	}

	return job, nil
}
