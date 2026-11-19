package media

// CreateUploadRequest defines the payload to initiate a direct media upload session.
type CreateUploadRequest struct {
	CourseID      string `json:"courseId" binding:"required"`
	FileName      string `json:"fileName" binding:"required"`
	DeclaredBytes int64  `json:"declaredBytes" binding:"required"`
}

// CompleteUploadRequest defines probed media information to finish an upload session.
type CompleteUploadRequest struct {
	IdempotencyKey  string `json:"idempotencyKey" binding:"required"`
	Container       string `json:"container" binding:"required"`
	Codec           string `json:"codec" binding:"required"`
	DurationSeconds int    `json:"durationSeconds" binding:"required"`
	Width           int    `json:"width" binding:"required"`
	Height          int    `json:"height" binding:"required"`
	FPS             int    `json:"fps" binding:"required"`
	ActualBytes     int64  `json:"actualBytes" binding:"required"`
}

// PublishMediaRequest specifies the ready media version to associate with a lesson.
type PublishMediaRequest struct {
	MediaVersionID string `json:"mediaVersionId" binding:"required"`
}

// AuthorizePlaybackRequest specifies the purpose and access token request for a lesson.
type AuthorizePlaybackRequest struct {
	CourseID string `json:"courseId" binding:"required"`
	Purpose  string `json:"purpose"` // "playback", "trial"
}
