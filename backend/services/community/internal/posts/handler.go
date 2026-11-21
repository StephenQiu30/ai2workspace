package posts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/auth"
	"github.com/gin-gonic/gin"
)

// ReplyNotifier delivers deduplicated in-app notifications when a comment or reply is created.
type ReplyNotifier func(ctx context.Context, eventID, recipientID, notifType, title, summary, targetType, targetID string) error

// Handler manages post, topic, and comment HTTP endpoints.
type Handler struct {
	store         Store
	resolver      auth.SessionResolver
	courseChecker auth.CourseAccessChecker
	notifyReply   ReplyNotifier
}

// NewHandler creates a posts Handler with trusted authentication, course access checking, and reply notifications.
func NewHandler(
	store Store,
	resolver auth.SessionResolver,
	courseChecker auth.CourseAccessChecker,
	notifyReply ReplyNotifier,
) *Handler {
	return &Handler{
		store:         store,
		resolver:      resolver,
		courseChecker: courseChecker,
		notifyReply:   notifyReply,
	}
}

func (h *Handler) checkCourseAccess(ctx context.Context, courseID, userID string) (bool, error) {
	if h.courseChecker == nil {
		return false, nil
	}
	return h.courseChecker(ctx, courseID, userID)
}

// CreatePost creates a new discussion post.
// @ID communityCreatePost
// @Summary Create discussion post
// @Tags Community
// @Accept json
// @Produce json
// @Param request body CreatePostRequest true "Post payload"
// @Success 201 {object} PostResponse
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 503 {object} auth.ErrorResponse
// @Router /api/community/posts [post]
func (h *Handler) CreatePost(c *gin.Context) {
	actor, ok := auth.RequireVerified(c, h.resolver)
	if !ok {
		return
	}

	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	scope := PostScope(strings.TrimSpace(req.Scope))
	if scope == "" {
		scope = ScopeGlobal
	}
	if scope != ScopeGlobal && scope != ScopeCourse {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_scope", Message: ErrInvalidScope.Error()})
		return
	}

	courseID := strings.TrimSpace(req.CourseID)
	if scope == ScopeCourse {
		if courseID == "" {
			c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "course_id_required", Message: ErrCourseIDRequired.Error()})
			return
		}
		allowed, err := h.checkCourseAccess(c.Request.Context(), courseID, actor.UserID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, auth.ErrorResponse{Error: "course_access_unavailable", Message: "course access check unavailable"})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "course_access_denied", Message: "current course access is required to post in course discussions"})
			return
		}
	}

	status := PostStatus(strings.TrimSpace(req.Status))
	if status == "" {
		status = StatusPublished
	}

	// Never derive publicSummary from body; publicSummary must be explicitly maintained.
	post, err := h.store.CreatePost(c.Request.Context(), CreatePostInput{
		Scope:          scope,
		CourseID:       courseID,
		Topic:          req.Topic,
		AuthorID:       actor.UserID,
		AuthorNickname: actor.DisplayName(),
		Title:          req.Title,
		PublicSummary:  req.PublicSummary,
		Body:           req.Body,
		Status:         status,
		RequestID:      req.RequestID,
	})
	if err != nil {
		writePostError(c, err)
		return
	}

	c.JSON(http.StatusCreated, postToResponse(post))
}

// GetPost retrieves post details with strict guest/course visibility boundaries and ordered comments.
// @ID communityGetPost
// @Summary Get post details
// @Tags Community
// @Produce json
// @Param id path string true "Post ID"
// @Success 200 {object} PostDetailResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Failure 410 {object} PostUnavailableResponse
// @Router /api/community/posts/{id} [get]
func (h *Handler) GetPost(c *gin.Context) {
	postID := c.Param("id")
	post, err := h.store.GetPost(c.Request.Context(), postID)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "post_not_found", Message: "post not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "internal_error", Message: "failed to load post"})
		return
	}

	switch post.Status {
	case StatusDeleted:
		c.JSON(http.StatusGone, PostUnavailableResponse{
			ID:          post.ID,
			Status:      string(StatusDeleted),
			AccessState: string(StatusDeleted),
			Error:       "post_deleted",
			Message:     "作者已删除",
		})
		return
	case StatusHidden:
		c.JSON(http.StatusGone, PostUnavailableResponse{
			ID:          post.ID,
			Status:      string(StatusHidden),
			AccessState: string(StatusHidden),
			Error:       "post_hidden",
			Message:     "内容已隐藏",
		})
		return
	}

	actor, authErr := auth.OptionalActor(c, h.resolver)
	if authErr != nil {
		if errors.Is(authErr, auth.ErrAccountSuspended) {
			auth.WriteAuthError(c, authErr)
			return
		}
		// Fail closed on upstream auth failure: treat caller as guest for public fields.
		actor = nil
	}

	if post.Status == StatusDraft {
		if actor == nil || actor.UserID != post.AuthorID {
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "post_not_found", Message: "post not found"})
			return
		}
		c.JSON(http.StatusOK, PostDetailResponse{
			ID:             post.ID,
			Scope:          string(post.Scope),
			CourseID:       post.CourseID,
			Topic:          post.Topic,
			AuthorID:       post.AuthorID,
			AuthorNickname: post.AuthorNickname,
			Title:          post.Title,
			PublicSummary:  post.PublicSummary,
			Body:           post.Body,
			Status:         string(post.Status),
			AccessState:    string(AccessFull),
			Version:        post.Version,
			Comments:       []CommentResponse{},
			CreatedAt:      post.CreatedAt,
			UpdatedAt:      post.UpdatedAt,
		})
		return
	}

	accessState := AccessFull
	if actor == nil {
		accessState = AccessLoginRequired
	} else if post.Scope == ScopeCourse {
		allowed, err := h.checkCourseAccess(c.Request.Context(), post.CourseID, actor.UserID)
		if err != nil || !allowed {
			accessState = AccessCourseAccessRequired
		}
	}

	body := ""
	commentResp := []CommentResponse{}
	if accessState == AccessFull {
		body = post.Body
		comments, err := h.store.ListComments(c.Request.Context(), post.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "internal_error", Message: "failed to load comments"})
			return
		}
		commentResp = make([]CommentResponse, 0, len(comments))
		for _, cmt := range comments {
			commentResp = append(commentResp, commentToResponse(cmt))
		}
	}

	c.JSON(http.StatusOK, PostDetailResponse{
		ID:             post.ID,
		Scope:          string(post.Scope),
		CourseID:       post.CourseID,
		Topic:          post.Topic,
		AuthorID:       post.AuthorID,
		AuthorNickname: post.AuthorNickname,
		Title:          post.Title,
		PublicSummary:  post.PublicSummary,
		Body:           body,
		Status:         string(post.Status),
		AccessState:    string(accessState),
		Version:        post.Version,
		Comments:       commentResp,
		CreatedAt:      post.CreatedAt,
		UpdatedAt:      post.UpdatedAt,
	})
}

// UpdatePost updates an author's post with optimistic locking.
// @ID communityUpdatePost
// @Summary Update post content
// @Tags Community
// @Accept json
// @Produce json
// @Param id path string true "Post ID"
// @Param request body UpdatePostRequest true "Update payload"
// @Success 200 {object} PostResponse
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Failure 409 {object} auth.ErrorResponse
// @Failure 410 {object} auth.ErrorResponse
// @Router /api/community/posts/{id} [patch]
func (h *Handler) UpdatePost(c *gin.Context) {
	actor, ok := auth.RequireVerified(c, h.resolver)
	if !ok {
		return
	}

	postID := c.Param("id")
	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	existing, err := h.store.GetPost(c.Request.Context(), postID)
	if err != nil {
		writePostError(c, err)
		return
	}
	if existing.Scope == ScopeCourse {
		allowed, err := h.checkCourseAccess(c.Request.Context(), existing.CourseID, actor.UserID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, auth.ErrorResponse{Error: "course_access_unavailable", Message: "course access check unavailable"})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "course_access_denied", Message: "current course access is required"})
			return
		}
	}

	post, err := h.store.UpdatePost(c.Request.Context(), UpdatePostInput{
		PostID:          postID,
		AuthorID:        actor.UserID,
		ExpectedVersion: req.ExpectedVersion,
		Topic:           req.Topic,
		Title:           req.Title,
		PublicSummary:   req.PublicSummary,
		Body:            req.Body,
		Status:          PostStatus(strings.TrimSpace(req.Status)),
	})
	if err != nil {
		writePostError(c, err)
		return
	}

	c.JSON(http.StatusOK, postToResponse(post))
}

// DeletePost logically deletes the author's post.
// @ID communityDeletePost
// @Summary Delete post
// @Tags Community
// @Produce json
// @Param id path string true "Post ID"
// @Success 204
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Failure 410 {object} auth.ErrorResponse
// @Router /api/community/posts/{id} [delete]
func (h *Handler) DeletePost(c *gin.Context) {
	actor, ok := auth.RequireVerified(c, h.resolver)
	if !ok {
		return
	}

	postID := c.Param("id")
	if err := h.store.DeletePost(c.Request.Context(), postID, actor.UserID); err != nil {
		writePostError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ListPosts lists published posts in reverse chronological order without exposing protected body.
// @ID communityListPosts
// @Summary List community posts
// @Tags Community
// @Produce json
// @Param scope query string false "Scope: global (default) or course"
// @Param courseId query string false "Course ID (required when scope=course)"
// @Param topic query string false "Topic slug filter"
// @Param page query int false "Page number (default 1)"
// @Param pageSize query int false "Page size (default 20, max 100)"
// @Success 200 {object} PostListResponse
// @Failure 400 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 503 {object} auth.ErrorResponse
// @Router /api/community/posts [get]
func (h *Handler) ListPosts(c *gin.Context) {
	scopeStr := strings.TrimSpace(c.Query("scope"))
	scope := ScopeGlobal
	if scopeStr != "" {
		scope = PostScope(scopeStr)
	}
	if scope != ScopeGlobal && scope != ScopeCourse {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_scope", Message: ErrInvalidScope.Error()})
		return
	}

	courseID := strings.TrimSpace(c.Query("courseId"))
	if scope == ScopeCourse {
		if courseID == "" {
			c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "course_id_required", Message: ErrCourseIDRequired.Error()})
			return
		}
		actor, err := auth.ResolveActor(c, h.resolver)
		if err != nil {
			if errors.Is(err, auth.ErrDependencyUnavailable) {
				auth.WriteAuthError(c, err)
				return
			}
			c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "course_access_denied", Message: "course discussion access requires authentication and course entitlement"})
			return
		}
		allowed, err := h.checkCourseAccess(c.Request.Context(), courseID, actor.UserID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, auth.ErrorResponse{Error: "course_access_unavailable", Message: "course access check unavailable"})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "course_access_denied", Message: "current course access is required to view course discussions"})
			return
		}
	}

	topic := strings.TrimSpace(c.Query("topic"))
	if topic != "" {
		if _, err := NormalizeTopic(topic); err != nil {
			c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_topic", Message: err.Error()})
			return
		}
	}

	page, pageSize := parsePagination(c)

	items, total, err := h.store.ListPosts(c.Request.Context(), ListPostsFilter{
		Scope:    scope,
		CourseID: courseID,
		Topic:    topic,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writePostError(c, err)
		return
	}

	respList := make([]PostResponse, 0, len(items))
	for _, p := range items {
		respList = append(respList, postToResponse(p))
	}

	c.JSON(http.StatusOK, PostListResponse{
		Items:    respList,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// ListTopics lists community topics along with published global post counts.
// @ID communityListTopics
// @Summary List community topics
// @Tags Community
// @Produce json
// @Success 200 {object} TopicListResponse
// @Router /api/community/topics [get]
func (h *Handler) ListTopics(c *gin.Context) {
	topics, err := h.store.ListTopics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "internal_error", Message: "failed to load topics"})
		return
	}
	c.JSON(http.StatusOK, TopicListResponse{Items: topics})
}

// CreateComment posts a top-level comment or single-level reply under a discussion thread.
// @ID communityCreateComment
// @Summary Add comment or reply to post
// @Tags Community
// @Accept json
// @Produce json
// @Param id path string true "Post ID"
// @Param request body CreateCommentRequest true "Comment payload"
// @Success 201 {object} CommentResponse
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Failure 503 {object} auth.ErrorResponse
// @Router /api/community/posts/{id}/comments [post]
func (h *Handler) CreateComment(c *gin.Context) {
	actor, ok := auth.RequireVerified(c, h.resolver)
	if !ok {
		return
	}

	postID := c.Param("id")
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	post, err := h.store.GetPost(c.Request.Context(), postID)
	if err != nil {
		writePostError(c, err)
		return
	}
	if post.Status != StatusPublished {
		c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "post_unavailable", Message: ErrParentPostHidden.Error()})
		return
	}
	if post.Scope == ScopeCourse {
		allowed, err := h.checkCourseAccess(c.Request.Context(), post.CourseID, actor.UserID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, auth.ErrorResponse{Error: "course_access_unavailable", Message: "course access check unavailable"})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "course_access_denied", Message: "current course access is required to comment on course discussions"})
			return
		}
	}

	comment, recipientID, err := h.store.CreateComment(c.Request.Context(), CreateCommentInput{
		PostID:          postID,
		AuthorID:        actor.UserID,
		AuthorNickname:  actor.DisplayName(),
		ParentCommentID: strings.TrimSpace(req.ParentCommentID),
		Body:            req.Body,
		RequestID:       req.RequestID,
	})
	if err != nil {
		writePostError(c, err)
		return
	}

	if h.notifyReply != nil && recipientID != "" && recipientID != actor.UserID {
		eventID := "comment_reply:" + comment.ID
		title := "收到新的社区回复"
		summary := fmt.Sprintf("%s 回复了讨论《%s》", actor.DisplayName(), post.Title)
		if err := h.notifyReply(c.Request.Context(), eventID, recipientID, "reply", title, summary, "post", post.ID); err != nil {
			slog.Warn("failed to deliver comment reply notification", "commentId", comment.ID, "recipientId", recipientID, "error", err)
		}
	}

	c.JSON(http.StatusCreated, commentToResponse(comment))
}

// UpdateComment updates an author's comment using optimistic locking.
// @ID communityUpdateComment
// @Summary Update comment content
// @Tags Community
// @Accept json
// @Produce json
// @Param id path string true "Comment ID"
// @Param request body UpdateCommentRequest true "Comment update payload"
// @Success 200 {object} CommentResponse
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Failure 409 {object} auth.ErrorResponse
// @Router /api/community/comments/{id} [patch]
func (h *Handler) UpdateComment(c *gin.Context) {
	actor, ok := auth.RequireVerified(c, h.resolver)
	if !ok {
		return
	}

	commentID := c.Param("id")
	var req UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	existing, err := h.store.GetComment(c.Request.Context(), commentID)
	if err != nil {
		writePostError(c, err)
		return
	}
	post, err := h.store.GetPost(c.Request.Context(), existing.PostID)
	if err != nil || post.Status != StatusPublished {
		c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "post_unavailable", Message: ErrParentPostHidden.Error()})
		return
	}
	if post.Scope == ScopeCourse {
		allowed, err := h.checkCourseAccess(c.Request.Context(), post.CourseID, actor.UserID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, auth.ErrorResponse{Error: "course_access_unavailable", Message: "course access check unavailable"})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "course_access_denied", Message: "current course access is required"})
			return
		}
	}

	updated, err := h.store.UpdateComment(c.Request.Context(), UpdateCommentInput{
		CommentID:       commentID,
		AuthorID:        actor.UserID,
		ExpectedVersion: req.ExpectedVersion,
		Body:            req.Body,
	})
	if err != nil {
		writePostError(c, err)
		return
	}

	c.JSON(http.StatusOK, commentToResponse(updated))
}

// DeleteComment logically deletes the author's comment.
// @ID communityDeleteComment
// @Summary Delete comment
// @Tags Community
// @Produce json
// @Param id path string true "Comment ID"
// @Success 204
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Router /api/community/comments/{id} [delete]
func (h *Handler) DeleteComment(c *gin.Context) {
	actor, ok := auth.RequireVerified(c, h.resolver)
	if !ok {
		return
	}

	commentID := c.Param("id")
	if err := h.store.DeleteComment(c.Request.Context(), commentID, actor.UserID); err != nil {
		writePostError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	sizeParam := c.Query("pageSize")
	if sizeParam == "" {
		sizeParam = c.DefaultQuery("limit", "20")
	}
	pageSize, _ := strconv.Atoi(sizeParam)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func writePostError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPermissionDenied):
		c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "forbidden", Message: err.Error()})
	case errors.Is(err, ErrVersionConflict):
		c.JSON(http.StatusConflict, auth.ErrorResponse{Error: "version_conflict", Message: err.Error()})
	case errors.Is(err, ErrPostDeleted):
		c.JSON(http.StatusGone, auth.ErrorResponse{Error: "post_deleted", Message: "作者已删除"})
	case errors.Is(err, ErrPostHidden):
		c.JSON(http.StatusGone, auth.ErrorResponse{Error: "post_hidden", Message: "内容已隐藏"})
	case errors.Is(err, ErrPostNotFound), errors.Is(err, ErrCommentNotFound), errors.Is(err, ErrParentPostHidden):
		c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "not_found", Message: err.Error()})
	case errors.Is(err, ErrPublicSummaryRequired):
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "public_summary_required", Message: err.Error()})
	case errors.Is(err, ErrDeepReplyNotAllowed),
		errors.Is(err, ErrInvalidParentPost),
		errors.Is(err, ErrInvalidScope),
		errors.Is(err, ErrInvalidTopic),
		errors.Is(err, ErrInvalidStatus),
		errors.Is(err, ErrCourseIDRequired),
		errors.Is(err, ErrEmptyContent):
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "internal_error", Message: err.Error()})
	}
}

func postToResponse(p *Post) PostResponse {
	return PostResponse{
		ID:             p.ID,
		Scope:          string(p.Scope),
		CourseID:       p.CourseID,
		Topic:          p.Topic,
		AuthorID:       p.AuthorID,
		AuthorNickname: p.AuthorNickname,
		Title:          p.Title,
		PublicSummary:  p.PublicSummary,
		Status:         string(p.Status),
		Version:        p.Version,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func commentToResponse(cmt *Comment) CommentResponse {
	return CommentResponse{
		ID:              cmt.ID,
		PostID:          cmt.PostID,
		AuthorID:        cmt.AuthorID,
		AuthorNickname:  cmt.AuthorNickname,
		ParentCommentID: cmt.ParentCommentID,
		Body:            cmt.Body,
		Status:          string(cmt.Status),
		Version:         cmt.Version,
		CreatedAt:       cmt.CreatedAt,
		UpdatedAt:       cmt.UpdatedAt,
	}
}
