package posts

import (
	"errors"
	"time"
)

var (
	// ErrPostNotFound indicates the requested post does not exist.
	ErrPostNotFound = errors.New("post not found")
	// ErrPostDeleted indicates the post was logically deleted by its author.
	ErrPostDeleted = errors.New("post deleted by author")
	// ErrPostHidden indicates the post was hidden by moderation.
	ErrPostHidden = errors.New("post hidden by moderation")
	// ErrCommentNotFound indicates the requested comment does not exist.
	ErrCommentNotFound = errors.New("comment not found")
	// ErrPermissionDenied indicates the caller does not own or have access to the resource.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrVersionConflict indicates an optimistic locking version mismatch.
	ErrVersionConflict = errors.New("version conflict: stale update")
	// ErrDeepReplyNotAllowed indicates replies beyond single level are rejected.
	ErrDeepReplyNotAllowed = errors.New("nested replies beyond single level are not allowed")
	// ErrParentPostHidden indicates comments cannot be added to hidden, deleted, or draft posts.
	ErrParentPostHidden = errors.New("cannot comment on unpublished, hidden, or deleted post")
	// ErrInvalidParentPost indicates the parent comment belongs to a different post.
	ErrInvalidParentPost = errors.New("parent comment does not belong to this post")
	// ErrInvalidScope indicates an unrecognized post scope.
	ErrInvalidScope = errors.New("scope must be global or course")
	// ErrInvalidTopic indicates an unrecognized topic slug.
	ErrInvalidTopic = errors.New("invalid topic slug")
	// ErrInvalidStatus indicates an unsupported post status transition or value.
	ErrInvalidStatus = errors.New("invalid post status")
	// ErrCourseIDRequired indicates courseId is mandatory for course-scoped posts.
	ErrCourseIDRequired = errors.New("courseId is required when scope is course")
	// ErrPublicSummaryRequired indicates publicSummary must be explicitly provided and maintained independently from body.
	ErrPublicSummaryRequired = errors.New("publicSummary is required and must be maintained independently from body")
	// ErrEmptyContent indicates required title or body is empty after trimming.
	ErrEmptyContent = errors.New("title and body must not be empty")
)

// PostScope indicates whether a post belongs to the global community or a course discussion board.
type PostScope string

const (
	ScopeGlobal PostScope = "global"
	ScopeCourse PostScope = "course"
)

// PostStatus defines the lifecycle state of a post or comment.
type PostStatus string

const (
	StatusDraft     PostStatus = "draft"
	StatusPublished PostStatus = "published"
	StatusHidden    PostStatus = "hidden"
	StatusDeleted   PostStatus = "deleted"
)

// AccessState describes the viewer's read access level for a post detail response.
type AccessState string

const (
	AccessFull                 AccessState = "full"
	AccessLoginRequired        AccessState = "login_required"
	AccessCourseAccessRequired AccessState = "course_access_required"
)

// TopicInfo describes a community topic category and its published global post count.
type TopicInfo struct {
	Slug        string `json:"slug" example:"learning-method"`
	Name        string `json:"name" example:"学习方法"`
	Description string `json:"description" example:"高效学习策略、笔记体系与复习实践交流"`
	PostCount   int    `json:"postCount" example:"12"`
}

// Post represents a community discussion thread.
type Post struct {
	ID             string     `json:"id"`
	Scope          PostScope  `json:"scope"`
	CourseID       string     `json:"courseId,omitempty"`
	Topic          string     `json:"topic"`
	AuthorID       string     `json:"authorId"`
	AuthorNickname string     `json:"authorNickname"`
	Title          string     `json:"title"`
	PublicSummary  string     `json:"publicSummary"`
	Body           string     `json:"body"`
	Status         PostStatus `json:"status"`
	Version        int        `json:"version"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// Comment represents a top-level comment or single-level reply under a post.
type Comment struct {
	ID              string     `json:"id"`
	PostID          string     `json:"postId"`
	AuthorID        string     `json:"authorId"`
	AuthorNickname  string     `json:"authorNickname"`
	ParentCommentID string     `json:"parentCommentId,omitempty"`
	Body            string     `json:"body"`
	Status          PostStatus `json:"status"`
	Version         int        `json:"version"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// CreatePostInput captures validated parameters for creating a post.
type CreatePostInput struct {
	Scope          PostScope
	CourseID       string
	Topic          string
	AuthorID       string
	AuthorNickname string
	Title          string
	PublicSummary  string
	Body           string
	Status         PostStatus
	RequestID      string
}

// UpdatePostInput captures validated parameters for updating a post.
type UpdatePostInput struct {
	PostID          string
	AuthorID        string
	ExpectedVersion int
	Topic           string
	Title           string
	PublicSummary   string
	Body            string
	Status          PostStatus
}

// ListPostsFilter filters and paginates published posts.
type ListPostsFilter struct {
	Scope    PostScope
	CourseID string
	Topic    string
	Page     int
	PageSize int
}

// CreateCommentInput captures validated parameters for creating a comment or reply.
type CreateCommentInput struct {
	PostID          string
	AuthorID        string
	AuthorNickname  string
	ParentCommentID string
	Body            string
	RequestID       string
}

// UpdateCommentInput captures validated parameters for updating a comment.
type UpdateCommentInput struct {
	CommentID       string
	AuthorID        string
	ExpectedVersion int
	Body            string
}
