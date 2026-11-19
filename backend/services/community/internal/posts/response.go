package posts

import "time"

// CommentResponse represents a serialized comment.
type CommentResponse struct {
	ID              string    `json:"id"`
	PostID          string    `json:"postId"`
	AuthorID        string    `json:"authorId"`
	ParentCommentID string    `json:"parentCommentId,omitempty"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"createdAt"`
}

// PostResponse represents serialized post item in list or basic view.
type PostResponse struct {
	ID            string    `json:"id"`
	Scope         string    `json:"scope"`
	CourseID      string    `json:"courseId,omitempty"`
	AuthorID      string    `json:"authorId"`
	Title         string    `json:"title"`
	PublicSummary string    `json:"publicSummary"`
	Status        string    `json:"status"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// PostDetailResponse represents comprehensive post view with protected body and comments.
type PostDetailResponse struct {
	ID            string            `json:"id"`
	Scope         string            `json:"scope"`
	CourseID      string            `json:"courseId,omitempty"`
	AuthorID      string            `json:"authorId"`
	Title         string            `json:"title"`
	PublicSummary string            `json:"publicSummary"`
	Body          string            `json:"body"`
	Status        string            `json:"status"`
	Version       int               `json:"version"`
	Comments      []CommentResponse `json:"comments"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

// PostListResponse represents paginated posts list.
type PostListResponse struct {
	Items []PostResponse `json:"items"`
	Total int            `json:"total"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
}
