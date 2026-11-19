package posts

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler manages post and comment HTTP endpoints.
type Handler struct {
	store Store
}

// NewHandler creates a posts Handler.
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// CreatePost creates a new discussion topic.
// @ID communityCreatePost
// @Summary Create discussion post
// @Tags Community
// @Accept json
// @Produce json
// @Param request body CreatePostRequest true "Post payload"
// @Success 201 {object} PostResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/community/posts [post]
func (h *Handler) CreatePost(c *gin.Context) {
	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	authorID := c.GetHeader("X-User-ID")
	if authorID == "" {
		authorID = "mock_author"
	}

	scope := ScopeGlobal
	if req.Scope == "course" {
		scope = ScopeCourse
	}

	summary := req.PublicSummary
	if summary == "" && len(req.Body) > 0 {
		runes := []rune(req.Body)
		if len(runes) > 100 {
			summary = string(runes[:100]) + "..."
		} else {
			summary = req.Body
		}
	}

	post, err := h.store.CreatePost(c.Request.Context(), scope, req.CourseID, authorID, req.Title, summary, req.Body)
	if err != nil {
		switch {
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, postToResponse(post))
}

// GetPost retrieves a post and its comments with protected body visibility.
// @ID communityGetPost
// @Summary Get post details
// @Tags Community
// @Produce json
// @Param id path string true "Post ID"
// @Success 200 {object} PostDetailResponse
// @Failure 404 {object} map[string]string
// @Router /api/community/posts/{id} [get]
func (h *Handler) GetPost(c *gin.Context) {
	postID := c.Param("id")
	viewerID := c.GetHeader("X-User-ID")

	post, comments, err := h.store.GetPost(c.Request.Context(), postID, viewerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	commentResp := make([]CommentResponse, 0, len(comments))
	for _, cmt := range comments {
		commentResp = append(commentResp, CommentResponse{
			ID:              cmt.ID,
			PostID:          cmt.PostID,
			AuthorID:        cmt.AuthorID,
			ParentCommentID: cmt.ParentCommentID,
			Body:            cmt.Body,
			CreatedAt:       cmt.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, PostDetailResponse{
		ID:            post.ID,
		Scope:         string(post.Scope),
		CourseID:      post.CourseID,
		AuthorID:      post.AuthorID,
		Title:         post.Title,
		PublicSummary: post.PublicSummary,
		Body:          post.Body,
		Status:        string(post.Status),
		Version:       post.Version,
		Comments:      commentResp,
		CreatedAt:     post.CreatedAt,
		UpdatedAt:     post.UpdatedAt,
	})
}

// UpdatePost updates author's post.
// @ID communityUpdatePost
// @Summary Update post content
// @Tags Community
// @Accept json
// @Produce json
// @Param id path string true "Post ID"
// @Param request body UpdatePostRequest true "Update payload"
// @Success 200 {object} PostResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/community/posts/{id} [patch]
func (h *Handler) UpdatePost(c *gin.Context) {
	postID := c.Param("id")
	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	authorID := c.GetHeader("X-User-ID")
	if authorID == "" {
		authorID = "mock_author"
	}

	post, err := h.store.UpdatePost(c.Request.Context(), postID, authorID, req.ExpectedVersion, req.Title, req.PublicSummary, req.Body)
	if err != nil {
		switch {
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPostNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, postToResponse(post))
}

// DeletePost marks author's post as deleted.
// @ID communityDeletePost
// @Summary Delete post
// @Tags Community
// @Produce json
// @Param id path string true "Post ID"
// @Success 204
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/community/posts/{id} [delete]
func (h *Handler) DeletePost(c *gin.Context) {
	postID := c.Param("id")
	authorID := c.GetHeader("X-User-ID")
	if authorID == "" {
		authorID = "mock_author"
	}

	if err := h.store.DeletePost(c.Request.Context(), postID, authorID); err != nil {
		switch {
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPostNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// ListPosts lists public post summaries.
// @ID communityListPosts
// @Summary List community posts
// @Tags Community
// @Produce json
// @Param scope query string false "Scope: global or course"
// @Param courseId query string false "Course ID"
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} PostListResponse
// @Router /api/community/posts [get]
func (h *Handler) ListPosts(c *gin.Context) {
	scopeStr := c.Query("scope")
	scope := ScopeGlobal
	if scopeStr == "course" {
		scope = ScopeCourse
	}

	courseID := c.Query("courseId")
	viewerID := c.GetHeader("X-User-ID")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	items, total, err := h.store.ListPosts(c.Request.Context(), scope, courseID, viewerID, page, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	respList := make([]PostResponse, 0, len(items))
	for _, p := range items {
		respList = append(respList, postToResponse(p))
	}

	c.JSON(http.StatusOK, PostListResponse{
		Items: respList,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// CreateComment posts a single-level response to a discussion thread.
// @ID communityCreateComment
// @Summary Add comment to post
// @Tags Community
// @Accept json
// @Produce json
// @Param id path string true "Post ID"
// @Param request body CreateCommentRequest true "Comment payload"
// @Success 201 {object} CommentResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/community/posts/{id}/comments [post]
func (h *Handler) CreateComment(c *gin.Context) {
	postID := c.Param("id")
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	authorID := c.GetHeader("X-User-ID")
	if authorID == "" {
		authorID = "mock_author"
	}

	comment, err := h.store.CreateComment(c.Request.Context(), postID, authorID, req.Body, req.ParentCommentID)
	if err != nil {
		switch {
		case errors.Is(err, ErrDeepReplyNotAllowed), errors.Is(err, ErrInvalidParentPost):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrParentPostHidden), errors.Is(err, ErrCommentNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, CommentResponse{
		ID:              comment.ID,
		PostID:          comment.PostID,
		AuthorID:        comment.AuthorID,
		ParentCommentID: comment.ParentCommentID,
		Body:            comment.Body,
		CreatedAt:       comment.CreatedAt,
	})
}

// DeleteComment deletes author's comment.
// @ID communityDeleteComment
// @Summary Delete comment
// @Tags Community
// @Produce json
// @Param id path string true "Comment ID"
// @Success 204
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/community/comments/{id} [delete]
func (h *Handler) DeleteComment(c *gin.Context) {
	commentID := c.Param("id")
	authorID := c.GetHeader("X-User-ID")
	if authorID == "" {
		authorID = "mock_author"
	}

	if err := h.store.DeleteComment(c.Request.Context(), commentID, authorID); err != nil {
		switch {
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCommentNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func postToResponse(p *Post) PostResponse {
	return PostResponse{
		ID:            p.ID,
		Scope:         string(p.Scope),
		CourseID:      p.CourseID,
		AuthorID:      p.AuthorID,
		Title:         p.Title,
		PublicSummary: p.PublicSummary,
		Status:        string(p.Status),
		Version:       p.Version,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}
