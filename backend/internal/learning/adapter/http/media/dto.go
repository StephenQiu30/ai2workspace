package media

import (
	"time"

	mediadomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/media"
)

// CreateUploadRequest defines the payload to initiate a direct media upload session.
type CreateUploadRequest struct {
	CourseID      string `json:"courseId" binding:"required" example:"crs_12345"`
	FileName      string `json:"fileName" binding:"required" example:"lesson1.mp4"`
	DeclaredBytes int64  `json:"declaredBytes" binding:"required" example:"104857600"`
}

// CompleteUploadRequest defines probed media information to finish an upload session.
type CompleteUploadRequest struct {
	IdempotencyKey  string `json:"idempotencyKey" binding:"required" example:"idemp-1"`
	Container       string `json:"container" binding:"required" example:"mp4"`
	Codec           string `json:"codec" binding:"required" example:"h264"`
	DurationSeconds int    `json:"durationSeconds" binding:"required" example:"600"`
	Width           int    `json:"width" binding:"required" example:"1920"`
	Height          int    `json:"height" binding:"required" example:"1080"`
	FPS             int    `json:"fps" binding:"required" example:"30"`
	ActualBytes     int64  `json:"actualBytes" binding:"required" example:"104857600"`
}

// PublishMediaRequest specifies the ready media version to associate with a lesson.
type PublishMediaRequest struct {
	MediaVersionID string `json:"mediaVersionId" binding:"required" example:"mv_12345"`
}

// AuthorizePlaybackRequest specifies the course context and requested purpose for playback.
type AuthorizePlaybackRequest struct {
	CourseID string `json:"courseId" binding:"required" example:"crs_12345"`
	Purpose  string `json:"purpose" example:"playback"` // "playback", "trial"
}

// UploadSessionResponse represents the response when an upload session is created or fetched.
type UploadSessionResponse struct {
	ID                 string                         `json:"id"`
	InstructorID       string                         `json:"instructorId"`
	CourseID           string                         `json:"courseId"`
	LessonID           string                         `json:"lessonId"`
	FileName           string                         `json:"fileName"`
	DeclaredBytes      int64                          `json:"declaredBytes"`
	ActualBytes        int64                          `json:"actualBytes"`
	Status             string                         `json:"status"`
	SourceVersion      int                            `json:"sourceVersion"`
	ExpiresAt          time.Time                      `json:"expiresAt"`
	UploadInstructions mediadomain.UploadInstructions `json:"uploadInstructions"`
	MediaVersionID     string                         `json:"mediaVersionId,omitempty"`
}

// MediaVersionResponse represents the details of a verified transcode media output.
type MediaVersionResponse struct {
	ID              string    `json:"id"`
	UploadID        string    `json:"uploadId"`
	LessonID        string    `json:"lessonId"`
	Status          string    `json:"status"`
	DurationSeconds int       `json:"durationSeconds"`
	Width           int       `json:"width"`
	Height          int       `json:"height"`
	FPS             int       `json:"fps"`
	Format          string    `json:"format"`
	PlaybackURL     string    `json:"playbackUrl"`
	HLSQualities    []string  `json:"hlsQualities"`
	FailureReason   string    `json:"failureReason,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// MediaJobResponse represents the status of an asynchronous processing job.
type MediaJobResponse struct {
	ID             string    `json:"id"`
	UploadID       string    `json:"uploadId"`
	MediaVersionID string    `json:"mediaVersionId"`
	Status         string    `json:"status"`
	RetryCount     int       `json:"retryCount"`
	ErrorMessage   string    `json:"errorMessage,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// PlaybackAuthorizationResponse provides a short-lived signed access authorization for HLS streaming.
type PlaybackAuthorizationResponse struct {
	MediaVersionID string    `json:"mediaVersionId"`
	PlaybackURL    string    `json:"playbackUrl"`
	HLSQualities   []string  `json:"hlsQualities"`
	ExpiresAt      time.Time `json:"expiresAt"`
	Token          string    `json:"token"`
}

// PublishMediaResponse confirms media version publication for a lesson.
type PublishMediaResponse struct {
	Status         string `json:"status" example:"published"`
	LessonID       string `json:"lessonId" example:"lsn_12345"`
	MediaVersionID string `json:"mediaVersionId" example:"mv_12345"`
}
