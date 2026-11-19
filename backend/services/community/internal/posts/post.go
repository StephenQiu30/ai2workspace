// Package posts manages community discussions, course boards, comments, and visibility.
package posts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrPostNotFound        = errors.New("post not found or hidden")
	ErrCommentNotFound     = errors.New("comment not found")
	ErrPermissionDenied    = errors.New("permission denied")
	ErrVersionConflict     = errors.New("version conflict: stale post update")
	ErrDeepReplyNotAllowed = errors.New("nested replies beyond single level are not allowed")
	ErrParentPostHidden    = errors.New("cannot comment on hidden or deleted post")
	ErrInvalidParentPost   = errors.New("parent comment does not belong to this post")
)

// PostScope indicates whether a post is on global community or course discussion.
type PostScope string

const (
	ScopeGlobal PostScope = "global"
	ScopeCourse PostScope = "course"
)

// PostStatus defines lifecycle of a post.
type PostStatus string

const (
	StatusPublished PostStatus = "published"
	StatusHidden    PostStatus = "hidden"
	StatusDeleted   PostStatus = "deleted"
)

// Post represents a discussion thread.
type Post struct {
	ID            string     `json:"id"`
	Scope         PostScope  `json:"scope"`
	CourseID      string     `json:"courseId,omitempty"`
	AuthorID      string     `json:"authorId"`
	Title         string     `json:"title"`
	PublicSummary string     `json:"publicSummary"`
	Body          string     `json:"body"`
	Status        PostStatus `json:"status"`
	Version       int        `json:"version"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// Comment represents a response or single-level reply under a post.
type Comment struct {
	ID              string     `json:"id"`
	PostID          string     `json:"postId"`
	AuthorID        string     `json:"authorId"`
	ParentCommentID string     `json:"parentCommentId,omitempty"`
	Body            string     `json:"body"`
	Status          PostStatus `json:"status"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// CourseAccessChecker verifies student access to course discussion boards.
type CourseAccessChecker func(courseID, userID string) (bool, error)

// Store defines persistence operations for posts and comments.
type Store interface {
	CreatePost(ctx context.Context, scope PostScope, courseID, authorID, title, publicSummary, body string) (*Post, error)
	GetPost(ctx context.Context, postID, viewerID string) (*Post, []*Comment, error)
	UpdatePost(ctx context.Context, postID, authorID string, expectedVersion int, title, publicSummary, body string) (*Post, error)
	DeletePost(ctx context.Context, postID, authorID string) error
	HidePost(ctx context.Context, postID string) error
	ListPosts(ctx context.Context, scope PostScope, courseID, viewerID string, page, limit int) ([]*Post, int, error)
	CreateComment(ctx context.Context, postID, authorID, body, parentCommentID string) (*Comment, error)
	DeleteComment(ctx context.Context, commentID, authorID string) error
	HideComment(ctx context.Context, commentID string) error
}

// MemoryStore provides in-memory store for posts and comments.
type MemoryStore struct {
	mu            sync.RWMutex
	posts         map[string]*Post    // postID -> Post
	comments      map[string]*Comment // commentID -> Comment
	accessChecker CourseAccessChecker
}

// NewMemoryStore initializes a new MemoryStore for posts.
func NewMemoryStore(checker CourseAccessChecker) *MemoryStore {
	return &MemoryStore{
		posts:         make(map[string]*Post),
		comments:      make(map[string]*Comment),
		accessChecker: checker,
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (s *MemoryStore) CreatePost(ctx context.Context, scope PostScope, courseID, authorID, title, publicSummary, body string) (*Post, error) {
	if scope == ScopeCourse && s.accessChecker != nil {
		allowed, err := s.accessChecker(courseID, authorID)
		if err != nil || !allowed {
			return nil, ErrPermissionDenied
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	postID := generateID("post")
	p := &Post{
		ID:            postID,
		Scope:         scope,
		CourseID:      courseID,
		AuthorID:      authorID,
		Title:         title,
		PublicSummary: publicSummary,
		Body:          body,
		Status:        StatusPublished,
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.posts[postID] = p
	return p, nil
}

func (s *MemoryStore) GetPost(ctx context.Context, postID, viewerID string) (*Post, []*Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	post, ok := s.posts[postID]
	if !ok || post.Status != StatusPublished {
		return nil, nil, ErrPostNotFound
	}

	// Determine view rights
	canViewBody := true
	if viewerID == "" {
		canViewBody = false
	} else if post.Scope == ScopeCourse && s.accessChecker != nil {
		allowed, err := s.accessChecker(post.CourseID, viewerID)
		if err != nil || !allowed {
			canViewBody = false
		}
	}

	// Prepare safe post view
	safePost := *post
	if !canViewBody {
		safePost.Body = "" // Protect full text for unauthorized users
	}

	// Fetch active comments if authorized
	var postComments []*Comment
	if canViewBody {
		for _, c := range s.comments {
			if c.PostID == postID && c.Status == StatusPublished {
				postComments = append(postComments, c)
			}
		}
	}

	return &safePost, postComments, nil
}

func (s *MemoryStore) UpdatePost(ctx context.Context, postID, authorID string, expectedVersion int, title, publicSummary, body string) (*Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	post, ok := s.posts[postID]
	if !ok || post.Status != StatusPublished {
		return nil, ErrPostNotFound
	}

	if post.AuthorID != authorID {
		return nil, ErrPermissionDenied
	}

	if expectedVersion > 0 && post.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	post.Title = title
	post.PublicSummary = publicSummary
	post.Body = body
	post.Version++
	post.UpdatedAt = time.Now()

	return post, nil
}

func (s *MemoryStore) DeletePost(ctx context.Context, postID, authorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	post, ok := s.posts[postID]
	if !ok || post.Status == StatusDeleted {
		return ErrPostNotFound
	}

	if post.AuthorID != authorID {
		return ErrPermissionDenied
	}

	post.Status = StatusDeleted
	post.UpdatedAt = time.Now()
	return nil
}

func (s *MemoryStore) HidePost(ctx context.Context, postID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	post, ok := s.posts[postID]
	if !ok {
		return ErrPostNotFound
	}

	post.Status = StatusHidden
	post.UpdatedAt = time.Now()
	return nil
}

func (s *MemoryStore) ListPosts(ctx context.Context, scope PostScope, courseID, viewerID string, page, limit int) ([]*Post, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var matched []*Post
	for _, p := range s.posts {
		if p.Status != StatusPublished {
			continue
		}
		if scope != "" && p.Scope != scope {
			continue
		}
		if scope == ScopeCourse && p.CourseID != courseID {
			continue
		}

		safe := *p
		// In lists, full body is not exposed to reduce payload and protect access
		safe.Body = ""
		matched = append(matched, &safe)
	}

	total := len(matched)
	start := (page - 1) * limit
	if start >= total {
		return []*Post{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	return matched[start:end], total, nil
}

func (s *MemoryStore) CreateComment(ctx context.Context, postID, authorID, body, parentCommentID string) (*Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	post, ok := s.posts[postID]
	if !ok || post.Status != StatusPublished {
		return nil, ErrParentPostHidden
	}

	// Verify single level reply rule
	if parentCommentID != "" {
		parent, exists := s.comments[parentCommentID]
		if !exists || parent.Status != StatusPublished {
			return nil, ErrCommentNotFound
		}
		if parent.PostID != postID {
			return nil, ErrInvalidParentPost
		}
		// Single level check: If parent itself has a parentCommentId, deep reply is rejected
		if parent.ParentCommentID != "" {
			return nil, ErrDeepReplyNotAllowed
		}
	}

	now := time.Now()
	commentID := generateID("cmt")
	c := &Comment{
		ID:              commentID,
		PostID:          postID,
		AuthorID:        authorID,
		ParentCommentID: parentCommentID,
		Body:            body,
		Status:          StatusPublished,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	s.comments[commentID] = c
	return c, nil
}

func (s *MemoryStore) DeleteComment(ctx context.Context, commentID, authorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.comments[commentID]
	if !ok || c.Status == StatusDeleted {
		return ErrCommentNotFound
	}
	if c.AuthorID != authorID {
		return ErrPermissionDenied
	}
	c.Status = StatusDeleted
	c.UpdatedAt = time.Now()
	return nil
}

func (s *MemoryStore) HideComment(ctx context.Context, commentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.comments[commentID]
	if !ok {
		return ErrCommentNotFound
	}
	c.Status = StatusHidden
	c.UpdatedAt = time.Now()
	return nil
}
