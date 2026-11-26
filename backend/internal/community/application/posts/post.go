package posts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	postsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/posts"
)

var topicCatalog = []postsdomain.TopicInfo{
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
		Name:        "课程咨询",
		Description: "选课、课程内容与学习问题",
	},
	{
		Slug:        "project-practice",
		Name:        "项目实战",
		Description: "作品制作、项目实践与复盘经验",
	},
	{
		Slug:        "services",
		Name:        "服务发布",
		Description: "创作服务、交付内容与合作咨询",
	},
	{
		Slug:        "resources",
		Name:        "资料分享",
		Description: "公开学习资料、工具清单与延伸阅读推荐",
	},
}

// DefaultTopics returns a fresh copy of the canonical topic catalog.
func DefaultTopics() []postsdomain.TopicInfo {
	out := make([]postsdomain.TopicInfo, len(topicCatalog))
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
	return "", postsdomain.ErrInvalidTopic
}

// Store defines persistence operations for posts, topics, and comments.
type Store interface {
	CreatePost(ctx context.Context, input postsdomain.CreatePostInput) (*postsdomain.Post, error)
	GetPost(ctx context.Context, postID string) (*postsdomain.Post, error)
	UpdatePost(ctx context.Context, input postsdomain.UpdatePostInput) (*postsdomain.Post, error)
	DeletePost(ctx context.Context, postID, authorID string) error
	HidePost(ctx context.Context, postID string) error
	ListPosts(ctx context.Context, filter postsdomain.ListPostsFilter) ([]*postsdomain.Post, int, error)
	ListTopics(ctx context.Context) ([]postsdomain.TopicInfo, error)

	CreateComment(ctx context.Context, input postsdomain.CreateCommentInput) (*postsdomain.Comment, string, error)
	GetComment(ctx context.Context, commentID string) (*postsdomain.Comment, error)
	ListComments(ctx context.Context, postID string) ([]*postsdomain.Comment, error)
	UpdateComment(ctx context.Context, input postsdomain.UpdateCommentInput) (*postsdomain.Comment, error)
	DeleteComment(ctx context.Context, commentID, authorID string) error
	HideComment(ctx context.Context, commentID string) error
}

// MemoryStore provides a thread-safe in-memory store for posts and comments.
type MemoryStore struct {
	mu            sync.RWMutex
	posts         map[string]*postsdomain.Post
	postByReq     map[string]string // "authorID:requestID" -> postID
	comments      map[string]*postsdomain.Comment
	commentByReq  map[string]string // "authorID:requestID" -> commentID
	lastTimestamp time.Time
}

// NewMemoryStore initializes a new MemoryStore for posts and comments.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		posts:        make(map[string]*postsdomain.Post),
		postByReq:    make(map[string]string),
		comments:     make(map[string]*postsdomain.Comment),
		commentByReq: make(map[string]string),
	}
}

func NewID(prefix string) string {
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

func clonePost(p *postsdomain.Post) *postsdomain.Post {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

func cloneComment(c *postsdomain.Comment) *postsdomain.Comment {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

func ValidateCreatePostInput(input *postsdomain.CreatePostInput) error {
	if input.Scope == "" {
		input.Scope = postsdomain.ScopeGlobal
	}
	if input.Scope != postsdomain.ScopeGlobal && input.Scope != postsdomain.ScopeCourse {
		return postsdomain.ErrInvalidScope
	}
	input.CourseID = strings.TrimSpace(input.CourseID)
	if input.Scope == postsdomain.ScopeCourse && input.CourseID == "" {
		return postsdomain.ErrCourseIDRequired
	}
	if input.Scope == postsdomain.ScopeGlobal {
		input.CourseID = ""
	}
	topic, err := NormalizeTopic(input.Topic)
	if err != nil {
		return err
	}
	input.Topic = topic

	if input.Status == "" {
		input.Status = postsdomain.StatusPublished
	}
	if input.Status != postsdomain.StatusPublished && input.Status != postsdomain.StatusDraft {
		return postsdomain.ErrInvalidStatus
	}

	input.Title = strings.TrimSpace(input.Title)
	input.PublicSummary = strings.TrimSpace(input.PublicSummary)
	input.Body = strings.TrimSpace(input.Body)
	if input.Title == "" || input.Body == "" {
		return postsdomain.ErrEmptyContent
	}
	if input.Status == postsdomain.StatusPublished && input.PublicSummary == "" {
		return postsdomain.ErrPublicSummaryRequired
	}
	return nil
}

// CreatePost persists a new post with optional idempotency via RequestID.
func (s *MemoryStore) CreatePost(ctx context.Context, input postsdomain.CreatePostInput) (*postsdomain.Post, error) {
	if err := ValidateCreatePostInput(&input); err != nil {
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
	postID := NewID("post")
	p := &postsdomain.Post{
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
func (s *MemoryStore) GetPost(ctx context.Context, postID string) (*postsdomain.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	post, ok := s.posts[postID]
	if !ok {
		return nil, postsdomain.ErrPostNotFound
	}
	return clonePost(post), nil
}

// UpdatePost updates an author's draft or published post with optimistic locking.
func (s *MemoryStore) UpdatePost(ctx context.Context, input postsdomain.UpdatePostInput) (*postsdomain.Post, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.PublicSummary = strings.TrimSpace(input.PublicSummary)
	input.Body = strings.TrimSpace(input.Body)
	if input.Title == "" || input.Body == "" {
		return nil, postsdomain.ErrEmptyContent
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	post, ok := s.posts[input.PostID]
	if !ok {
		return nil, postsdomain.ErrPostNotFound
	}
	if post.Status == postsdomain.StatusDeleted {
		return nil, postsdomain.ErrPostDeleted
	}
	if post.Status == postsdomain.StatusHidden {
		return nil, postsdomain.ErrPostHidden
	}
	if post.AuthorID != input.AuthorID {
		return nil, postsdomain.ErrPermissionDenied
	}
	if input.ExpectedVersion <= 0 || post.Version != input.ExpectedVersion {
		return nil, postsdomain.ErrVersionConflict
	}

	nextStatus := post.Status
	if input.Status != "" {
		if input.Status != postsdomain.StatusDraft && input.Status != postsdomain.StatusPublished {
			return nil, postsdomain.ErrInvalidStatus
		}
		nextStatus = input.Status
	}
	if nextStatus == postsdomain.StatusPublished && input.PublicSummary == "" {
		return nil, postsdomain.ErrPublicSummaryRequired
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
		return postsdomain.ErrPostNotFound
	}
	if post.AuthorID != authorID {
		return postsdomain.ErrPermissionDenied
	}
	if post.Status == postsdomain.StatusHidden {
		return postsdomain.ErrPostHidden
	}
	if post.Status == postsdomain.StatusDeleted {
		return nil
	}

	post.Status = postsdomain.StatusDeleted
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
		return postsdomain.ErrPostNotFound
	}
	post.Status = postsdomain.StatusHidden
	post.Version++
	post.UpdatedAt = s.nextTimestampLocked()
	return nil
}

// ListPosts returns published posts matching filter in deterministic reverse chronological order.
// Returned posts never include full Body content.
func (s *MemoryStore) ListPosts(ctx context.Context, filter postsdomain.ListPostsFilter) ([]*postsdomain.Post, int, error) {
	if filter.Scope == "" {
		filter.Scope = postsdomain.ScopeGlobal
	}
	if filter.Scope != postsdomain.ScopeGlobal && filter.Scope != postsdomain.ScopeCourse {
		return nil, 0, postsdomain.ErrInvalidScope
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

	var matched []*postsdomain.Post
	for _, p := range s.posts {
		if p.Status != postsdomain.StatusPublished {
			continue
		}
		if p.Scope != filter.Scope {
			continue
		}
		if filter.Scope == postsdomain.ScopeCourse && p.CourseID != filter.CourseID {
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
		return []*postsdomain.Post{}, total, nil
	}
	end := start + filter.PageSize
	if end > total {
		end = total
	}
	return matched[start:end], total, nil
}

// ListTopics returns the canonical topics with counts of published global posts.
func (s *MemoryStore) ListTopics(ctx context.Context) ([]postsdomain.TopicInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := make(map[string]int, len(topicCatalog))
	for _, p := range s.posts {
		if p.Status == postsdomain.StatusPublished && p.Scope == postsdomain.ScopeGlobal {
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
func (s *MemoryStore) CreateComment(ctx context.Context, input postsdomain.CreateCommentInput) (*postsdomain.Comment, string, error) {
	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" {
		return nil, "", postsdomain.ErrEmptyContent
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
		return nil, "", postsdomain.ErrPostNotFound
	}
	if post.Status != postsdomain.StatusPublished {
		return nil, "", postsdomain.ErrParentPostHidden
	}

	notifyRecipient := post.AuthorID
	if input.ParentCommentID != "" {
		parent, exists := s.comments[input.ParentCommentID]
		if !exists || parent.Status != postsdomain.StatusPublished {
			return nil, "", postsdomain.ErrCommentNotFound
		}
		if parent.PostID != input.PostID {
			return nil, "", postsdomain.ErrInvalidParentPost
		}
		if parent.ParentCommentID != "" {
			return nil, "", postsdomain.ErrDeepReplyNotAllowed
		}
		notifyRecipient = parent.AuthorID
	}

	now := s.nextTimestampLocked()
	commentID := NewID("cmt")
	c := &postsdomain.Comment{
		ID:              commentID,
		PostID:          input.PostID,
		AuthorID:        input.AuthorID,
		AuthorNickname:  input.AuthorNickname,
		ParentCommentID: input.ParentCommentID,
		Body:            input.Body,
		Status:          postsdomain.StatusPublished,
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
func (s *MemoryStore) GetComment(ctx context.Context, commentID string) (*postsdomain.Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.comments[commentID]
	if !ok {
		return nil, postsdomain.ErrCommentNotFound
	}
	return cloneComment(c), nil
}

// ListComments returns published comments for a post in deterministic ascending chronological order.
func (s *MemoryStore) ListComments(ctx context.Context, postID string) ([]*postsdomain.Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*postsdomain.Comment
	for _, c := range s.comments {
		if c.PostID == postID && c.Status == postsdomain.StatusPublished {
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
func (s *MemoryStore) UpdateComment(ctx context.Context, input postsdomain.UpdateCommentInput) (*postsdomain.Comment, error) {
	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" {
		return nil, postsdomain.ErrEmptyContent
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.comments[input.CommentID]
	if !ok {
		return nil, postsdomain.ErrCommentNotFound
	}
	if c.Status == postsdomain.StatusDeleted || c.Status == postsdomain.StatusHidden {
		return nil, postsdomain.ErrCommentNotFound
	}
	if c.AuthorID != input.AuthorID {
		return nil, postsdomain.ErrPermissionDenied
	}
	if input.ExpectedVersion <= 0 || c.Version != input.ExpectedVersion {
		return nil, postsdomain.ErrVersionConflict
	}

	post, ok := s.posts[c.PostID]
	if !ok || post.Status != postsdomain.StatusPublished {
		return nil, postsdomain.ErrParentPostHidden
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
		return postsdomain.ErrCommentNotFound
	}
	if c.AuthorID != authorID {
		return postsdomain.ErrPermissionDenied
	}
	if c.Status == postsdomain.StatusHidden {
		return postsdomain.ErrPostHidden
	}
	if c.Status == postsdomain.StatusDeleted {
		return nil
	}
	c.Status = postsdomain.StatusDeleted
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
		return postsdomain.ErrCommentNotFound
	}
	c.Status = postsdomain.StatusHidden
	c.Version++
	c.UpdatedAt = s.nextTimestampLocked()
	return nil
}
