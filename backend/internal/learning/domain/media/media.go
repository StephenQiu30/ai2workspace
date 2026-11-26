package media

import (
	"errors"
	"strings"
	"time"
)

var (
	// ErrInvalidPlaybackToken indicates a malformed or tampered playback token.
	ErrInvalidPlaybackToken = errors.New("invalid playback token")
	// ErrExpiredPlaybackToken indicates the playback token has expired.
	ErrExpiredPlaybackToken = errors.New("playback token has expired")
)

// PlaybackTokenClaims represents the signed claims embedded in a short-lived playback token.
type PlaybackTokenClaims struct {
	MediaVersionID string `json:"mv"`
	CourseID       string `json:"cid"`
	LessonID       string `json:"lid"`
	UserID         string `json:"uid"`
	Purpose        string `json:"pur"`
	ExpiresAt      int64  `json:"exp"`
}

// ValidateMedia enforces the MVP video upload baseline rules from the PRD (R-15).
func ValidateMedia(meta MediaMetadata) error {
	if meta.ActualBytes <= 0 {
		return ErrFileEmpty
	}
	if meta.ActualBytes > MaxDeclaredBytes {
		return ErrFileTooLarge
	}

	container := strings.ToLower(strings.TrimSpace(meta.Container))
	if container != "mp4" {
		return ErrInvalidContainer
	}

	codec := strings.ToLower(strings.TrimSpace(meta.Codec))
	if codec != "h264" && codec != "avc1" {
		return ErrInvalidCodec
	}

	if meta.DurationSeconds <= 0 {
		return ErrDurationZero
	}
	if meta.DurationSeconds > MaxDurationSeconds {
		return ErrDurationExceeded
	}

	longSide := meta.Width
	shortSide := meta.Height
	if shortSide > longSide {
		longSide, shortSide = shortSide, longSide
	}

	if longSide <= 0 || shortSide <= 0 || longSide > MaxLongSide || shortSide > MaxShortSide {
		return ErrResolutionExceeded
	}

	if meta.FPS <= 0 || meta.FPS > MaxFPS {
		return ErrFPSExceeded
	}

	return nil
}

// DetermineQualities selects HLS qualities without upsampling.
func DetermineQualities(width, height int) []string {
	longSide := width
	shortSide := height
	if shortSide > longSide {
		longSide, shortSide = shortSide, longSide
	}

	if longSide >= 1920 || shortSide >= 1080 {
		return []string{"720p", "1080p"}
	}
	if longSide >= 1280 || shortSide >= 720 {
		return []string{"720p"}
	}
	return []string{"source"}
}

var (
	ErrUploadNotFound       = errors.New("upload session not found")
	ErrUploadExpired        = errors.New("upload session has expired")
	ErrUploadCancelled      = errors.New("upload session has been cancelled")
	ErrUploadCompleted      = errors.New("upload session already completed and cannot be cancelled")
	ErrIdempotencyConflict  = errors.New("idempotency key reused with different payload")
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
	ErrLessonMismatch       = errors.New("media version does not belong to target lesson")
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
	PlaybackTokenTTL           = 30 * time.Minute
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
