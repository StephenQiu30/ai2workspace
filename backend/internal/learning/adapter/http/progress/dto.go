package progress

// SaveProgressRequest defines payload to update student study position.
type SaveProgressRequest struct {
	CourseID        string `json:"courseId" binding:"required" example:"crs_12345"`
	ContentVersion  int    `json:"contentVersion" binding:"required" example:"1"`
	PositionSeconds int    `json:"positionSeconds" example:"120"`
	ExpectedVersion int    `json:"expectedVersion" example:"0"`
}

// ProgressResponse represents serialized progress record.
type ProgressResponse struct {
	ID              string `json:"id"`
	UserID          string `json:"userId"`
	CourseID        string `json:"courseId"`
	LessonID        string `json:"lessonId"`
	ContentVersion  int    `json:"contentVersion"`
	PositionSeconds int    `json:"positionSeconds"`
	Version         int    `json:"version"`
	UpdatedAt       string `json:"updatedAt"`
}
