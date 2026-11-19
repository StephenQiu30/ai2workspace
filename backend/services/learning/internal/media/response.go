package media

import "time"

// UploadSessionResponse represents the response when an upload session is created or fetched.
type UploadSessionResponse struct {
	ID                 string             `json:"id"`
	InstructorID       string             `json:"instructorId"`
	CourseID           string             `json:"courseId"`
	LessonID           string             `json:"lessonId"`
	FileName           string             `json:"fileName"`
	DeclaredBytes      int64              `json:"declaredBytes"`
	ActualBytes        int64              `json:"actualBytes"`
	Status             string             `json:"status"`
	SourceVersion      int                `json:"sourceVersion"`
	ExpiresAt          time.Time          `json:"expiresAt"`
	UploadInstructions UploadInstructions `json:"uploadInstructions"`
	MediaVersionID     string             `json:"mediaVersionId,omitempty"`
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

// PlaybackAuthorizationResponse provides a short-lived access authorization for HLS streaming.
type PlaybackAuthorizationResponse struct {
	MediaVersionID string    `json:"mediaVersionId"`
	PlaybackURL    string    `json:"playbackUrl"`
	HLSQualities   []string  `json:"hlsQualities"`
	ExpiresAt      time.Time `json:"expiresAt"`
	Token          string    `json:"token"`
}
