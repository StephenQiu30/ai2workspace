// Package posts manages community discussions, course boards, topics, comments, and visibility.
package posts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
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

var topicCatalog = []TopicInfo{
	{
		Slug:        "general",
		Name:        "综合讨论",
		Description: "平台学习日常、学习心得与开放话题交流",
	},
	{
		Slug:        "learning-method",
		Name:        "学习方法",
		Description: "高效学习策略、笔记体系与复习实践交流",
	},
	{
		Slug:        "course-qa",
		Name:        "课程答疑",
		Description: "围绕课程知识点与练习题的提问与解答",
	},
	{
		Slug:        "project-practice",
		Name:        "项目实战",
		Description: "项目拆分、代码实现与复盘经验分享",
	},
	{
		Slug:        "resources",
		Name:        "资料分享",
		Description: "公开学习资料、工具清单与延伸阅读推荐",
	},
}

// DefaultTopics returns a fresh copy of the canonical topic catalog.
func DefaultTopics() []TopicInfo {
	out := make([]TopicInfo, len(topicCatalog))
	copy(out, topicCatalog)
	return out
}

// NormalizeTopic validates and normalizes a topic slug. Empty defaults to "general".
func NormalizeTopic(raw string) (string, error) {
	slug := strings.TrimSpace(raw)
	if slug == "" {
		return "general", nil
	}
	for _, t := range topicCatalog {
		if t.Slug == slug {
			return slug, nil
		}
	}
	return "", ErrInvalidTopic
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

// Store defines persistence operations for posts, topics, and comments.
type Store interface {
	CreatePost(ctx context.Context, input CreatePostInput) (*Post, error)
	GetPost(ctx context.Context, postID string) (*Post, error)
	UpdatePost(ctx context.Context, input UpdatePostInput) (*Post, error)
	DeletePost(ctx context.Context, postID, authorID string) error
	HidePost(ctx context.Context, postID string) error
	ListPosts(ctx context.Context, filter ListPostsFilter) ([]*Post, int, error)
	ListTopics(ctx context.Context) ([]TopicInfo, error)

	CreateComment(ctx context.Context, input CreateCommentInput) (*Comment, string, error)
	GetComment(ctx context.Context, commentID string) (*Comment, error)
	ListComments(ctx context.Context, postID string) ([]*Comment, error)
	UpdateComment(ctx context.Context, input UpdateCommentInput) (*Comment, error)
	DeleteComment(ctx context.Context, commentID, authorID string) error
	HideComment(ctx context.Context, commentID string) error
}

// MemoryStore provides a thread-safe in-memory store for posts and comments.
type MemoryStore struct {
	mu            sync.RWMutex
	posts         map[string]*Post
	postByReq     map[string]string // "authorID:requestID" -> postID
	comments      map[string]*Comment
	commentByReq  map[string]string // "authorID:requestID" -> commentID
	lastTimestamp time.Time
}

// NewMemoryStore initializes a new MemoryStore for posts and comments.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		posts:        make(map[string]*Post),
		postByReq:    make(map[string]string),
		comments:     make(map[string]*Comment),
		commentByReq: make(map[string]string),
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (s *MemoryStore) nextTimestampLocked() time.Time {
	now := time.Now().UTC()
	if !now.After(s.lastTimestamp) {
		now = s.lastTimestamp.Add(time.Microsecond)
	}
	s.lastTimestamp = now
	return now
}

func clonePost(p *Post) *Post {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

func cloneComment(c *Comment) *Comment {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

func validateCreatePostInput(input *CreatePostInput) error {
	if input.Scope == "" {
		input.Scope = ScopeGlobal
	}
	if input.Scope != ScopeGlobal && input.Scope != ScopeCourse {
		return ErrInvalidScope
	}
	input.CourseID = strings.TrimSpace(input.CourseID)
	if input.Scope == ScopeCourse && input.CourseID == "" {
		return ErrCourseIDRequired
	}
	if input.Scope == ScopeGlobal {
		input.CourseID = ""
	}
	topic, err := NormalizeTopic(input.Topic)
	if err != nil {
		return err
	}
	input.Topic = topic

	if input.Status == "" {
		input.Status = StatusPublished
	}
	if input.Status != StatusPublished && input.Status != StatusDraft {
		return ErrInvalidStatus
	}

	input.Title = strings.TrimSpace(input.Title)
	input.PublicSummary = strings.TrimSpace(input.PublicSummary)
	input.Body = strings.TrimSpace(input.Body)
	if input.Title == "" || input.Body == "" {
		return ErrEmptyContent
	}
	if input.Status == StatusPublished && input.PublicSummary == "" {
		return ErrPublicSummaryRequired
	}
	return nil
}

// CreatePost persists a new post with optional idempotency via RequestID.
func (s *MemoryStore) CreatePost(ctx context.Context, input CreatePostInput) (*Post, error) {
	if err := validateCreatePostInput(&input); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if reqID := strings.TrimSpace(input.RequestID); reqID != "" {
		key := input.AuthorID + ":" + reqID
		if existingID, ok := s.postByReq[key]; ok {
			return clonePost(s.posts[existingID]), nil
		}
	}

	now := s.nextTimestampLocked()
	postID := generateID("post")
	p := &Post{
		ID:             postID,
		Scope:          input.Scope,
		CourseID:       input.CourseID,
		Topic:          input.Topic,
		AuthorID:       input.AuthorID,
		AuthorNickname: input.AuthorNickname,
		Title:          input.Title,
		PublicSummary:  input.PublicSummary,
		Body:           input.Body,
		Status:         input.Status,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.posts[postID] = p
	if reqID := strings.TrimSpace(input.RequestID); reqID != "" {
		s.postByReq[input.AuthorID+":"+reqID] = postID
	}
	return clonePost(p), nil
}

// GetPost returns the raw post regardless of visibility status.
func (s *MemoryStore) GetPost(ctx context.Context, postID string) (*Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	post, ok := s.posts[postID]
	if !ok {
		return nil, ErrPostNotFound
	}
	return clonePost(post), nil
}

// UpdatePost updates an author's draft or published post with optimistic locking.
func (s *MemoryStore) UpdatePost(ctx context.Context, input UpdatePostInput) (*Post, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.PublicSummary = strings.TrimSpace(input.PublicSummary)
	input.Body = strings.TrimSpace(input.Body)
	if input.Title == "" || input.Body == "" {
		return nil, ErrEmptyContent
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	post, ok := s.posts[input.PostID]
	if !ok {
		return nil, ErrPostNotFound
	}
	if post.Status == StatusDeleted {
		return nil, ErrPostDeleted
	}
	if post.Status == StatusHidden {
		return nil, ErrPostHidden
	}
	if post.AuthorID != input.AuthorID {
		return nil, ErrPermissionDenied
	}
	if input.ExpectedVersion <= 0 || post.Version != input.ExpectedVersion {
		return nil, ErrVersionConflict
	}

	nextStatus := post.Status
	if input.Status != "" {
		if input.Status != StatusDraft && input.Status != StatusPublished {
			return nil, ErrInvalidStatus
		}
		nextStatus = input.Status
	}
	if nextStatus == StatusPublished && input.PublicSummary == "" {
		return nil, ErrPublicSummaryRequired
	}

	nextTopic := post.Topic
	if strings.TrimSpace(input.Topic) != "" {
		normalized, err := NormalizeTopic(input.Topic)
		if err != nil {
			return nil, err
		}
		nextTopic = normalized
	}

	post.Title = input.Title
	post.PublicSummary = input.PublicSummary
	post.Body = input.Body
	post.Topic = nextTopic
	post.Status = nextStatus
	post.Version++
	post.UpdatedAt = s.nextTimestampLocked()

	return clonePost(post), nil
}

// DeletePost logically deletes a post by its author. Repeated deletes by the author are idempotent.
func (s *MemoryStore) DeletePost(ctx context.Context, postID, authorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	post, ok := s.posts[postID]
	if !ok {
		return ErrPostNotFound
	}
	if post.AuthorID != authorID {
		return ErrPermissionDenied
	}
	if post.Status == StatusHidden {
		return ErrPostHidden
	}
	if post.Status == StatusDeleted {
		return nil
	}

	post.Status = StatusDeleted
	post.Version++
	post.UpdatedAt = s.nextTimestampLocked()
	return nil
}

// HidePost marks a post as hidden by moderation.
func (s *MemoryStore) HidePost(ctx context.Context, postID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	post, ok := s.posts[postID]
	if !ok {
		return ErrPostNotFound
	}
	post.Status = StatusHidden
	post.Version++
	post.UpdatedAt = s.nextTimestampLocked()
	return nil
}

// ListPosts returns published posts matching filter in deterministic reverse chronological order.
// Returned posts never include full Body content.
func (s *MemoryStore) ListPosts(ctx context.Context, filter ListPostsFilter) ([]*Post, int, error) {
	if filter.Scope == "" {
		filter.Scope = ScopeGlobal
	}
	if filter.Scope != ScopeGlobal && filter.Scope != ScopeCourse {
		return nil, 0, ErrInvalidScope
	}
	if filter.Topic != "" {
		if _, err := NormalizeTopic(filter.Topic); err != nil {
			return nil, 0, err
		}
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var matched []*Post
	for _, p := range s.posts {
		if p.Status != StatusPublished {
			continue
		}
		if p.Scope != filter.Scope {
			continue
		}
		if filter.Scope == ScopeCourse && p.CourseID != filter.CourseID {
			continue
		}
		if filter.Topic != "" && p.Topic != filter.Topic {
			continue
		}
		safe := clonePost(p)
		safe.Body = ""
		matched = append(matched, safe)
	}

	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].ID > matched[j].ID
	})

	total := len(matched)
	start := (filter.Page - 1) * filter.PageSize
	if start >= total {
		return []*Post{}, total, nil
	}
	end := start + filter.PageSize
	if end > total {
		end = total
	}
	return matched[start:end], total, nil
}

// ListTopics returns the canonical topics with counts of published global posts.
func (s *MemoryStore) ListTopics(ctx context.Context) ([]TopicInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := make(map[string]int, len(topicCatalog))
	for _, p := range s.posts {
		if p.Status == StatusPublished && p.Scope == ScopeGlobal {
			counts[p.Topic]++
		}
	}

	topics := DefaultTopics()
	for i := range topics {
		topics[i].PostCount = counts[topics[i].Slug]
	}
	return topics, nil
}

// CreateComment creates a top-level comment or single-level reply and returns the target recipient ID to notify.
func (s *MemoryStore) CreateComment(ctx context.Context, input CreateCommentInput) (*Comment, string, error) {
	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" {
		return nil, "", ErrEmptyContent
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if reqID := strings.TrimSpace(input.RequestID); reqID != "" {
		key := input.AuthorID + ":" + reqID
		if existingID, ok := s.commentByReq[key]; ok {
			existing := s.comments[existingID]
			recipient := ""
			if existing.ParentCommentID != "" {
				if parent, ok := s.comments[existing.ParentCommentID]; ok {
					recipient = parent.AuthorID
				}
			} else if p, ok := s.posts[existing.PostID]; ok {
				recipient = p.AuthorID
			}
			return cloneComment(existing), recipient, nil
		}
	}

	post, ok := s.posts[input.PostID]
	if !ok {
		return nil, "", ErrPostNotFound
	}
	if post.Status != StatusPublished {
		return nil, "", ErrParentPostHidden
	}

	notifyRecipient := post.AuthorID
	if input.ParentCommentID != "" {
		parent, exists := s.comments[input.ParentCommentID]
		if !exists || parent.Status != StatusPublished {
			return nil, "", ErrCommentNotFound
		}
		if parent.PostID != input.PostID {
			return nil, "", ErrInvalidParentPost
		}
		if parent.ParentCommentID != "" {
			return nil, "", ErrDeepReplyNotAllowed
		}
		notifyRecipient = parent.AuthorID
	}

	now := s.nextTimestampLocked()
	commentID := generateID("cmt")
	c := &Comment{
		ID:              commentID,
		PostID:          input.PostID,
		AuthorID:        input.AuthorID,
		AuthorNickname:  input.AuthorNickname,
		ParentCommentID: input.ParentCommentID,
		Body:            input.Body,
		Status:          StatusPublished,
		Version:         1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	s.comments[commentID] = c
	if reqID := strings.TrimSpace(input.RequestID); reqID != "" {
		s.commentByReq[input.AuthorID+":"+reqID] = commentID
	}
	return cloneComment(c), notifyRecipient, nil
}

// GetComment returns a raw comment by ID.
func (s *MemoryStore) GetComment(ctx context.Context, commentID string) (*Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.comments[commentID]
	if !ok {
		return nil, ErrCommentNotFound
	}
	return cloneComment(c), nil
}

// ListComments returns published comments for a post in deterministic ascending chronological order.
func (s *MemoryStore) ListComments(ctx context.Context, postID string) ([]*Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Comment
	for _, c := range s.comments {
		if c.PostID == postID && c.Status == StatusPublished {
			result = append(result, cloneComment(c))
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.Before(result[j].CreatedAt)
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// UpdateComment updates a comment by its author using optimistic locking.
func (s *MemoryStore) UpdateComment(ctx context.Context, input UpdateCommentInput) (*Comment, error) {
	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" {
		return nil, ErrEmptyContent
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.comments[input.CommentID]
	if !ok {
		return nil, ErrCommentNotFound
	}
	if c.Status == StatusDeleted || c.Status == StatusHidden {
		return nil, ErrCommentNotFound
	}
	if c.AuthorID != input.AuthorID {
		return nil, ErrPermissionDenied
	}
	if input.ExpectedVersion <= 0 || c.Version != input.ExpectedVersion {
		return nil, ErrVersionConflict
	}

	post, ok := s.posts[c.PostID]
	if !ok || post.Status != StatusPublished {
		return nil, ErrParentPostHidden
	}

	c.Body = input.Body
	c.Version++
	c.UpdatedAt = s.nextTimestampLocked()
	return cloneComment(c), nil
}

// DeleteComment logically deletes a comment by its author.
func (s *MemoryStore) DeleteComment(ctx context.Context, commentID, authorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.comments[commentID]
	if !ok {
		return ErrCommentNotFound
	}
	if c.AuthorID != authorID {
		return ErrPermissionDenied
	}
	if c.Status == StatusHidden {
		return ErrPostHidden
	}
	if c.Status == StatusDeleted {
		return nil
	}
	c.Status = StatusDeleted
	c.Version++
	c.UpdatedAt = s.nextTimestampLocked()
	return nil
}

// HideComment marks a comment as hidden by moderation.
func (s *MemoryStore) HideComment(ctx context.Context, commentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.comments[commentID]
	if !ok {
		return ErrCommentNotFound
	}
	c.Status = StatusHidden
	c.Version++
	c.UpdatedAt = s.nextTimestampLocked()
	return nil
}
