package posts

import (
	"time"

	postsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/posts"
)

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

// CommentResponse represents a serialized comment or single-level reply.
type CommentResponse struct {
	ID              string    `json:"id" example:"cmt_12345"`
	PostID          string    `json:"postId" example:"post_12345"`
	AuthorID        string    `json:"authorId" example:"usr_12345"`
	AuthorNickname  string    `json:"authorNickname" example:"小屿学员"`
	ParentCommentID string    `json:"parentCommentId,omitempty" example:"cmt_10001"`
	Body            string    `json:"body" example:"感谢分享！"`
	Status          string    `json:"status" example:"published"`
	Version         int       `json:"version" example:"1"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// PostResponse represents a serialized post item with public metadata only (never includes Body).
type PostResponse struct {
	ID             string    `json:"id" example:"post_12345"`
	Scope          string    `json:"scope" example:"global"`
	CourseID       string    `json:"courseId,omitempty" example:"crs_1001"`
	Topic          string    `json:"topic" example:"learning-method"`
	AuthorID       string    `json:"authorId" example:"usr_12345"`
	AuthorNickname string    `json:"authorNickname" example:"小屿学员"`
	Title          string    `json:"title" example:"如何系统整理课程笔记"`
	PublicSummary  string    `json:"publicSummary" example:"分享一套兼顾复盘与检索的课程学习笔记结构。"`
	Status         string    `json:"status" example:"published"`
	Version        int       `json:"version" example:"1"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// PostDetailResponse represents a post detail view with protected body and comments gated by accessState.
type PostDetailResponse struct {
	ID             string            `json:"id" example:"post_12345"`
	Scope          string            `json:"scope" example:"global"`
	CourseID       string            `json:"courseId,omitempty" example:"crs_1001"`
	Topic          string            `json:"topic" example:"learning-method"`
	AuthorID       string            `json:"authorId" example:"usr_12345"`
	AuthorNickname string            `json:"authorNickname" example:"小屿学员"`
	Title          string            `json:"title" example:"如何系统整理课程笔记"`
	PublicSummary  string            `json:"publicSummary" example:"分享一套兼顾复盘与检索的课程学习笔记结构。"`
	Body           string            `json:"body" example:"仅登录且具备权限后可见的正文内容"`
	Status         string            `json:"status" example:"published"`
	AccessState    string            `json:"accessState" example:"full"`
	Version        int               `json:"version" example:"1"`
	Comments       []CommentResponse `json:"comments"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

// PostUnavailableResponse describes a post that has been deleted by its author or hidden by moderation.
type PostUnavailableResponse struct {
	ID          string `json:"id" example:"post_12345"`
	Status      string `json:"status" example:"deleted"`
	AccessState string `json:"accessState" example:"deleted"`
	Error       string `json:"error" example:"post_deleted"`
	Message     string `json:"message" example:"作者已删除"`
}

// PostListResponse represents a paginated list of posts.
type PostListResponse struct {
	Items    []PostResponse `json:"items"`
	Total    int            `json:"total" example:"42"`
	Page     int            `json:"page" example:"1"`
	PageSize int            `json:"pageSize" example:"20"`
}

// TopicListResponse represents the list of community topics and published post counts.
type TopicListResponse struct {
	Items []postsdomain.TopicInfo `json:"items"`
}
