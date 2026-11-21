package posts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists posts and comments in PostgreSQL using pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a PostgreSQL-backed posts store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func nullableString(v string) any {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return v
}

// CreatePost inserts a new post with optional idempotency on (author_id, request_id).
func (s *PostgresStore) CreatePost(ctx context.Context, input CreatePostInput) (*Post, error) {
	if err := validateCreatePostInput(&input); err != nil {
		return nil, err
	}

	reqID := strings.TrimSpace(input.RequestID)
	if reqID != "" {
		existing, err := s.getPostByRequestID(ctx, input.AuthorID, reqID)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("lookup idempotent post: %w", err)
		}
	}

	now := time.Now().UTC()
	postID := generateID("post")
	var p Post
	var scopeStr, statusStr string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO posts (
			id, scope, course_id, topic, author_id, author_nickname,
			title, public_summary, body, status, version, request_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 1, $11, $12, $12)
		ON CONFLICT (author_id, request_id) WHERE request_id IS NOT NULL AND request_id <> ''
		DO UPDATE SET updated_at = posts.updated_at
		RETURNING id, scope, course_id, topic, author_id, author_nickname,
		          title, public_summary, body, status, version, created_at, updated_at
	`, postID, string(input.Scope), input.CourseID, input.Topic, input.AuthorID, input.AuthorNickname,
		input.Title, input.PublicSummary, input.Body, string(input.Status), nullableString(reqID), now,
	).Scan(
		&p.ID, &scopeStr, &p.CourseID, &p.Topic, &p.AuthorID, &p.AuthorNickname,
		&p.Title, &p.PublicSummary, &p.Body, &statusStr, &p.Version, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert post: %w", err)
	}
	p.Scope = PostScope(scopeStr)
	p.Status = PostStatus(statusStr)
	return &p, nil
}

func (s *PostgresStore) getPostByRequestID(ctx context.Context, authorID, requestID string) (*Post, error) {
	var p Post
	var scopeStr, statusStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, scope, course_id, topic, author_id, author_nickname,
		       title, public_summary, body, status, version, created_at, updated_at
		FROM posts
		WHERE author_id = $1 AND request_id = $2
	`, authorID, requestID).Scan(
		&p.ID, &scopeStr, &p.CourseID, &p.Topic, &p.AuthorID, &p.AuthorNickname,
		&p.Title, &p.PublicSummary, &p.Body, &statusStr, &p.Version, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.Scope = PostScope(scopeStr)
	p.Status = PostStatus(statusStr)
	return &p, nil
}

// GetPost retrieves a raw post by ID.
func (s *PostgresStore) GetPost(ctx context.Context, postID string) (*Post, error) {
	var p Post
	var scopeStr, statusStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, scope, course_id, topic, author_id, author_nickname,
		       title, public_summary, body, status, version, created_at, updated_at
		FROM posts
		WHERE id = $1
	`, postID).Scan(
		&p.ID, &scopeStr, &p.CourseID, &p.Topic, &p.AuthorID, &p.AuthorNickname,
		&p.Title, &p.PublicSummary, &p.Body, &statusStr, &p.Version, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		return nil, fmt.Errorf("get post: %w", err)
	}
	p.Scope = PostScope(scopeStr)
	p.Status = PostStatus(statusStr)
	return &p, nil
}

// UpdatePost updates an author's post with optimistic locking.
func (s *PostgresStore) UpdatePost(ctx context.Context, input UpdatePostInput) (*Post, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.PublicSummary = strings.TrimSpace(input.PublicSummary)
	input.Body = strings.TrimSpace(input.Body)
	if input.Title == "" || input.Body == "" {
		return nil, ErrEmptyContent
	}

	existing, err := s.GetPost(ctx, input.PostID)
	if err != nil {
		return nil, err
	}
	if existing.Status == StatusDeleted {
		return nil, ErrPostDeleted
	}
	if existing.Status == StatusHidden {
		return nil, ErrPostHidden
	}
	if existing.AuthorID != input.AuthorID {
		return nil, ErrPermissionDenied
	}
	if input.ExpectedVersion <= 0 || existing.Version != input.ExpectedVersion {
		return nil, ErrVersionConflict
	}

	nextStatus := existing.Status
	if input.Status != "" {
		if input.Status != StatusDraft && input.Status != StatusPublished {
			return nil, ErrInvalidStatus
		}
		nextStatus = input.Status
	}
	if nextStatus == StatusPublished && input.PublicSummary == "" {
		return nil, ErrPublicSummaryRequired
	}

	nextTopic := existing.Topic
	if strings.TrimSpace(input.Topic) != "" {
		normalized, err := NormalizeTopic(input.Topic)
		if err != nil {
			return nil, err
		}
		nextTopic = normalized
	}

	now := time.Now().UTC()
	var updated Post
	var scopeStr, statusStr string
	err = s.pool.QueryRow(ctx, `
		UPDATE posts
		SET title = $1, public_summary = $2, body = $3, topic = $4, status = $5,
		    version = version + 1, updated_at = $6
		WHERE id = $7 AND author_id = $8 AND version = $9 AND status IN ('draft', 'published')
		RETURNING id, scope, course_id, topic, author_id, author_nickname,
		          title, public_summary, body, status, version, created_at, updated_at
	`, input.Title, input.PublicSummary, input.Body, nextTopic, string(nextStatus), now,
		input.PostID, input.AuthorID, input.ExpectedVersion,
	).Scan(
		&updated.ID, &scopeStr, &updated.CourseID, &updated.Topic, &updated.AuthorID, &updated.AuthorNickname,
		&updated.Title, &updated.PublicSummary, &updated.Body, &statusStr, &updated.Version, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVersionConflict
		}
		return nil, fmt.Errorf("update post: %w", err)
	}
	updated.Scope = PostScope(scopeStr)
	updated.Status = PostStatus(statusStr)
	return &updated, nil
}

// DeletePost logically deletes a post by its author.
func (s *PostgresStore) DeletePost(ctx context.Context, postID, authorID string) error {
	existing, err := s.GetPost(ctx, postID)
	if err != nil {
		return err
	}
	if existing.AuthorID != authorID {
		return ErrPermissionDenied
	}
	if existing.Status == StatusHidden {
		return ErrPostHidden
	}
	if existing.Status == StatusDeleted {
		return nil
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE posts SET status = 'deleted', version = version + 1, updated_at = $1
		WHERE id = $2 AND author_id = $3 AND status <> 'hidden'
	`, time.Now().UTC(), postID, authorID)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	return nil
}

// HidePost marks a post as hidden by moderation.
func (s *PostgresStore) HidePost(ctx context.Context, postID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE posts SET status = 'hidden', version = version + 1, updated_at = $1
		WHERE id = $2
	`, time.Now().UTC(), postID)
	if err != nil {
		return fmt.Errorf("hide post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPostNotFound
	}
	return nil
}

// ListPosts lists published posts sorted by created_at DESC, id DESC without returning body.
func (s *PostgresStore) ListPosts(ctx context.Context, filter ListPostsFilter) ([]*Post, int, error) {
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

	conditions := []string{"status = 'published'", "scope = $1"}
	args := []any{string(filter.Scope)}
	argPos := 2

	if filter.Scope == ScopeCourse {
		conditions = append(conditions, fmt.Sprintf("course_id = $%d", argPos))
		args = append(args, filter.CourseID)
		argPos++
	}
	if filter.Topic != "" {
		conditions = append(conditions, fmt.Sprintf("topic = $%d", argPos))
		args = append(args, filter.Topic)
		argPos++
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM posts WHERE "+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}

	offset := (filter.Page - 1) * filter.PageSize
	query := fmt.Sprintf(`
		SELECT id, scope, course_id, topic, author_id, author_nickname,
		       title, public_summary, status, version, created_at, updated_at
		FROM posts
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argPos, argPos+1)
	args = append(args, filter.PageSize, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	items := make([]*Post, 0)
	for rows.Next() {
		var p Post
		var scopeStr, statusStr string
		if err := rows.Scan(
			&p.ID, &scopeStr, &p.CourseID, &p.Topic, &p.AuthorID, &p.AuthorNickname,
			&p.Title, &p.PublicSummary, &statusStr, &p.Version, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan post: %w", err)
		}
		p.Scope = PostScope(scopeStr)
		p.Status = PostStatus(statusStr)
		p.Body = ""
		items = append(items, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate posts: %w", err)
	}
	return items, total, nil
}

// ListTopics returns all topics with published global post counts.
func (s *PostgresStore) ListTopics(ctx context.Context) ([]TopicInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT topic, COUNT(*)
		FROM posts
		WHERE status = 'published' AND scope = 'global'
		GROUP BY topic
	`)
	if err != nil {
		return nil, fmt.Errorf("count posts by topic: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var slug string
		var cnt int
		if err := rows.Scan(&slug, &cnt); err != nil {
			return nil, fmt.Errorf("scan topic count: %w", err)
		}
		counts[slug] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate topic counts: %w", err)
	}

	topics := DefaultTopics()
	for i := range topics {
		topics[i].PostCount = counts[topics[i].Slug]
	}
	return topics, nil
}

// CreateComment inserts a top-level comment or single-level reply and returns the target recipient ID.
func (s *PostgresStore) CreateComment(ctx context.Context, input CreateCommentInput) (*Comment, string, error) {
	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" {
		return nil, "", ErrEmptyContent
	}

	post, err := s.GetPost(ctx, input.PostID)
	if err != nil {
		return nil, "", err
	}
	if post.Status != StatusPublished {
		return nil, "", ErrParentPostHidden
	}

	notifyRecipient := post.AuthorID
	if input.ParentCommentID != "" {
		parent, err := s.GetComment(ctx, input.ParentCommentID)
		if err != nil {
			return nil, "", err
		}
		if parent.Status != StatusPublished {
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

	reqID := strings.TrimSpace(input.RequestID)
	now := time.Now().UTC()
	commentID := generateID("cmt")

	var c Comment
	var parentID *string
	var statusStr string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO comments (
			id, post_id, author_id, author_nickname, parent_comment_id,
			body, status, version, request_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'published', 1, $7, $8, $8)
		ON CONFLICT (author_id, request_id) WHERE request_id IS NOT NULL AND request_id <> ''
		DO UPDATE SET updated_at = comments.updated_at
		RETURNING id, post_id, author_id, author_nickname, parent_comment_id,
		          body, status, version, created_at, updated_at
	`, commentID, input.PostID, input.AuthorID, input.AuthorNickname,
		nullableString(input.ParentCommentID), input.Body, nullableString(reqID), now,
	).Scan(
		&c.ID, &c.PostID, &c.AuthorID, &c.AuthorNickname, &parentID,
		&c.Body, &statusStr, &c.Version, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, "", fmt.Errorf("insert comment: %w", err)
	}
	if parentID != nil {
		c.ParentCommentID = *parentID
	}
	c.Status = PostStatus(statusStr)
	return &c, notifyRecipient, nil
}

// GetComment retrieves a raw comment by ID.
func (s *PostgresStore) GetComment(ctx context.Context, commentID string) (*Comment, error) {
	var c Comment
	var parentID *string
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, post_id, author_id, author_nickname, parent_comment_id,
		       body, status, version, created_at, updated_at
		FROM comments
		WHERE id = $1
	`, commentID).Scan(
		&c.ID, &c.PostID, &c.AuthorID, &c.AuthorNickname, &parentID,
		&c.Body, &statusStr, &c.Version, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCommentNotFound
		}
		return nil, fmt.Errorf("get comment: %w", err)
	}
	if parentID != nil {
		c.ParentCommentID = *parentID
	}
	c.Status = PostStatus(statusStr)
	return &c, nil
}

// ListComments retrieves published comments for a post ordered by created_at ASC, id ASC.
func (s *PostgresStore) ListComments(ctx context.Context, postID string) ([]*Comment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, post_id, author_id, author_nickname, parent_comment_id,
		       body, status, version, created_at, updated_at
		FROM comments
		WHERE post_id = $1 AND status = 'published'
		ORDER BY created_at ASC, id ASC
	`, postID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	result := make([]*Comment, 0)
	for rows.Next() {
		var c Comment
		var parentID *string
		var statusStr string
		if err := rows.Scan(
			&c.ID, &c.PostID, &c.AuthorID, &c.AuthorNickname, &parentID,
			&c.Body, &statusStr, &c.Version, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		if parentID != nil {
			c.ParentCommentID = *parentID
		}
		c.Status = PostStatus(statusStr)
		result = append(result, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comments: %w", err)
	}
	return result, nil
}

// UpdateComment updates an author's comment using optimistic locking.
func (s *PostgresStore) UpdateComment(ctx context.Context, input UpdateCommentInput) (*Comment, error) {
	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" {
		return nil, ErrEmptyContent
	}

	existing, err := s.GetComment(ctx, input.CommentID)
	if err != nil {
		return nil, err
	}
	if existing.Status != StatusPublished {
		return nil, ErrCommentNotFound
	}
	if existing.AuthorID != input.AuthorID {
		return nil, ErrPermissionDenied
	}
	if input.ExpectedVersion <= 0 || existing.Version != input.ExpectedVersion {
		return nil, ErrVersionConflict
	}

	post, err := s.GetPost(ctx, existing.PostID)
	if err != nil || post.Status != StatusPublished {
		return nil, ErrParentPostHidden
	}

	var updated Comment
	var parentID *string
	var statusStr string
	err = s.pool.QueryRow(ctx, `
		UPDATE comments
		SET body = $1, version = version + 1, updated_at = $2
		WHERE id = $3 AND author_id = $4 AND version = $5 AND status = 'published'
		RETURNING id, post_id, author_id, author_nickname, parent_comment_id,
		          body, status, version, created_at, updated_at
	`, input.Body, time.Now().UTC(), input.CommentID, input.AuthorID, input.ExpectedVersion,
	).Scan(
		&updated.ID, &updated.PostID, &updated.AuthorID, &updated.AuthorNickname, &parentID,
		&updated.Body, &statusStr, &updated.Version, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVersionConflict
		}
		return nil, fmt.Errorf("update comment: %w", err)
	}
	if parentID != nil {
		updated.ParentCommentID = *parentID
	}
	updated.Status = PostStatus(statusStr)
	return &updated, nil
}

// DeleteComment logically deletes an author's comment.
func (s *PostgresStore) DeleteComment(ctx context.Context, commentID, authorID string) error {
	existing, err := s.GetComment(ctx, commentID)
	if err != nil {
		return err
	}
	if existing.AuthorID != authorID {
		return ErrPermissionDenied
	}
	if existing.Status == StatusHidden {
		return ErrPostHidden
	}
	if existing.Status == StatusDeleted {
		return nil
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE comments SET status = 'deleted', version = version + 1, updated_at = $1
		WHERE id = $2 AND author_id = $3 AND status <> 'hidden'
	`, time.Now().UTC(), commentID, authorID)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	return nil
}

// HideComment marks a comment as hidden by moderation.
func (s *PostgresStore) HideComment(ctx context.Context, commentID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE comments SET status = 'hidden', version = version + 1, updated_at = $1
		WHERE id = $2
	`, time.Now().UTC(), commentID)
	if err != nil {
		return fmt.Errorf("hide comment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCommentNotFound
	}
	return nil
}
