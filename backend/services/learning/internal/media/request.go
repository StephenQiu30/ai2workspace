package media

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
