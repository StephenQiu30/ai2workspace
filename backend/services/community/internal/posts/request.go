package posts

// CreatePostRequest defines the payload to create a community discussion post.
type CreatePostRequest struct {
	Scope         string `json:"scope" example:"global"`
	CourseID      string `json:"courseId,omitempty" example:"crs_1001"`
	Topic         string `json:"topic,omitempty" example:"learning-method"`
	Title         string `json:"title" binding:"required" example:"如何系统整理课程笔记"`
	PublicSummary string `json:"publicSummary" example:"分享一套兼顾复盘与检索的课程学习笔记结构。"`
	Body          string `json:"body" binding:"required" example:"完整讨论正文内容"`
	Status        string `json:"status,omitempty" example:"published"`
	RequestID     string `json:"requestId,omitempty" example:"req_post_001"`
}

// UpdatePostRequest defines the payload to update an existing discussion post.
type UpdatePostRequest struct {
	ExpectedVersion int    `json:"expectedVersion" binding:"required,min=1" example:"1"`
	Topic           string `json:"topic,omitempty" example:"learning-method"`
	Title           string `json:"title" binding:"required" example:"如何系统整理课程笔记（修订版）"`
	PublicSummary   string `json:"publicSummary" example:"更新后的公开摘要说明。"`
	Body            string `json:"body" binding:"required" example:"更新后的正文内容"`
	Status          string `json:"status,omitempty" example:"published"`
}

// CreateCommentRequest defines the payload to add a comment or single-level reply.
type CreateCommentRequest struct {
	Body            string `json:"body" binding:"required" example:"感谢分享，很实用！"`
	ParentCommentID string `json:"parentCommentId,omitempty" example:"cmt_12345"`
	RequestID       string `json:"requestId,omitempty" example:"req_cmt_001"`
}

// UpdateCommentRequest defines the payload to update an existing comment with optimistic locking.
type UpdateCommentRequest struct {
	ExpectedVersion int    `json:"expectedVersion" binding:"required,min=1" example:"1"`
	Body            string `json:"body" binding:"required" example:"补充一条复习建议"`
}
