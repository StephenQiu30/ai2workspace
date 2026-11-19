package posts

// CreatePostRequest defines payload to publish a discussion topic.
type CreatePostRequest struct {
	Scope         string `json:"scope"` // "global", "course"
	CourseID      string `json:"courseId"`
	Title         string `json:"title" binding:"required"`
	PublicSummary string `json:"publicSummary"`
	Body          string `json:"body" binding:"required"`
}

// UpdatePostRequest defines payload to update existing discussion topic.
type UpdatePostRequest struct {
	ExpectedVersion int    `json:"expectedVersion"`
	Title           string `json:"title" binding:"required"`
	PublicSummary   string `json:"publicSummary"`
	Body            string `json:"body" binding:"required"`
}

// CreateCommentRequest defines payload to reply or comment on a post.
type CreateCommentRequest struct {
	Body            string `json:"body" binding:"required"`
	ParentCommentID string `json:"parentCommentId,omitempty"`
}
