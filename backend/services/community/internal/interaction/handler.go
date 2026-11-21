package interaction

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/posts"
	"github.com/gin-gonic/gin"
)

// SetReactionRequest defines payload to like, dislike, or cancel feedback.
type SetReactionRequest struct {
	TargetType string `json:"targetType" binding:"required" example:"post"`
	TargetID   string `json:"targetId" binding:"required" example:"post_12345"`
	Value      string `json:"value" binding:"required" example:"like"`
}

// SetFavoriteRequest defines payload to bookmark or unbookmark a post.
type SetFavoriteRequest struct {
	TargetType string `json:"targetType,omitempty" example:"post"`
	PostID     string `json:"postId" example:"post_12345"`
	TargetID   string `json:"targetId,omitempty" example:"post_12345"`
	Saved      bool   `json:"saved" example:"true"`
}

// FavoriteResponse represents a post bookmarking status.
type FavoriteResponse struct {
	PostID string `json:"postId" example:"post_12345"`
	Saved  bool   `json:"saved" example:"true"`
}

// Handler manages interaction HTTP endpoints.
type Handler struct {
	store         Store
	postStore     posts.Store
	resolver      auth.SessionResolver
	courseChecker auth.CourseAccessChecker
}

// NewHandler creates an interaction Handler with trusted auth and target validation.
func NewHandler(
	store Store,
	postStore posts.Store,
	resolver auth.SessionResolver,
	courseChecker auth.CourseAccessChecker,
) *Handler {
	return &Handler{
		store:         store,
		postStore:     postStore,
		resolver:      resolver,
		courseChecker: courseChecker,
	}
}

func (h *Handler) verifyCourseAccess(ctx context.Context, courseID, userID string) (bool, error) {
	if h.courseChecker == nil {
		return false, nil
	}
	return h.courseChecker(ctx, courseID, userID)
}

func (h *Handler) validateTargetAccess(c *gin.Context, userID, targetType, targetID string) bool {
	if h.postStore == nil {
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "internal_error", Message: "post store unavailable"})
		return false
	}

	var post *posts.Post
	switch targetType {
	case "post":
		p, err := h.postStore.GetPost(c.Request.Context(), targetID)
		if err != nil {
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "target_not_found", Message: "target post not found"})
			return false
		}
		if p.Status == posts.StatusDeleted {
			c.JSON(http.StatusGone, auth.ErrorResponse{Error: "target_deleted", Message: "作者已删除，无法互动"})
			return false
		}
		if p.Status == posts.StatusHidden {
			c.JSON(http.StatusGone, auth.ErrorResponse{Error: "target_hidden", Message: "内容已隐藏，无法互动"})
			return false
		}
		if p.Status != posts.StatusPublished {
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "target_not_found", Message: "target post unavailable"})
			return false
		}
		post = p
	case "comment":
		cmt, err := h.postStore.GetComment(c.Request.Context(), targetID)
		if err != nil {
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "target_not_found", Message: "target comment not found"})
			return false
		}
		if cmt.Status == posts.StatusDeleted || cmt.Status == posts.StatusHidden || cmt.Status != posts.StatusPublished {
			c.JSON(http.StatusGone, auth.ErrorResponse{Error: "target_unavailable", Message: "target comment is no longer available"})
			return false
		}
		p, err := h.postStore.GetPost(c.Request.Context(), cmt.PostID)
		if err != nil || p.Status != posts.StatusPublished {
			c.JSON(http.StatusGone, auth.ErrorResponse{Error: "target_unavailable", Message: "parent post is no longer available"})
			return false
		}
		post = p
	default:
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_target_type", Message: ErrInvalidTargetType.Error()})
		return false
	}

	if post.Scope == posts.ScopeCourse {
		if userID == "" {
			c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "course_access_denied", Message: "current course access is required"})
			return false
		}
		allowed, err := h.verifyCourseAccess(c.Request.Context(), post.CourseID, userID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, auth.ErrorResponse{Error: "course_access_unavailable", Message: "course access check unavailable"})
			return false
		}
		if !allowed {
			c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "course_access_denied", Message: "current course access is required"})
			return false
		}
	}
	return true
}

// SetReaction sets or toggles mutually exclusive like/dislike feedback on a post or comment.
// @ID communitySetReaction
// @Summary Set or toggle like/dislike reaction
// @Tags Community
// @Accept json
// @Produce json
// @Param request body SetReactionRequest true "Reaction payload"
// @Success 200 {object} ReactionSummary
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Failure 410 {object} auth.ErrorResponse
// @Router /api/community/reactions [put]
func (h *Handler) SetReaction(c *gin.Context) {
	actor, ok := auth.RequireVerified(c, h.resolver)
	if !ok {
		return
	}

	var req SetReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	targetType := strings.TrimSpace(req.TargetType)
	targetID := strings.TrimSpace(req.TargetID)
	val := ReactionValue(strings.TrimSpace(req.Value))

	if targetType != "post" && targetType != "comment" {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_target_type", Message: ErrInvalidTargetType.Error()})
		return
	}
	if !IsValidReactionValue(val) {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_reaction_value", Message: ErrInvalidReactionValue.Error()})
		return
	}

	if !h.validateTargetAccess(c, actor.UserID, targetType, targetID) {
		return
	}

	summary, err := h.store.SetReaction(c.Request.Context(), actor.UserID, targetType, targetID, val)
	if err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "reaction_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetReaction returns reaction counts and the viewer's reaction state for an accessible target.
// @ID communityGetReaction
// @Summary Get reaction summary for target
// @Tags Community
// @Produce json
// @Param targetType query string true "Target type: post or comment"
// @Param targetId query string true "Target ID"
// @Success 200 {object} ReactionSummary
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Failure 410 {object} auth.ErrorResponse
// @Router /api/community/reactions [get]
func (h *Handler) GetReaction(c *gin.Context) {
	targetType := strings.TrimSpace(c.Query("targetType"))
	targetID := strings.TrimSpace(c.Query("targetId"))
	if targetType != "post" && targetType != "comment" {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_target_type", Message: ErrInvalidTargetType.Error()})
		return
	}
	if targetID == "" {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: "targetId is required"})
		return
	}

	actor, authErr := auth.OptionalActor(c, h.resolver)
	if authErr != nil {
		if errors.Is(authErr, auth.ErrAccountSuspended) {
			auth.WriteAuthError(c, authErr)
			return
		}
		actor = nil
	}
	viewerID := ""
	if actor != nil {
		viewerID = actor.UserID
	}

	// Comments are protected behind login; guests may only inspect public global post reaction counts.
	if targetType == "comment" && viewerID == "" {
		c.JSON(http.StatusUnauthorized, auth.ErrorResponse{Error: "unauthorized", Message: "authentication required to inspect comment reactions"})
		return
	}

	if !h.validateTargetAccess(c, viewerID, targetType, targetID) {
		return
	}

	summary, err := h.store.GetReaction(c.Request.Context(), viewerID, targetType, targetID)
	if err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "reaction_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// SetFavorite bookmarks or unbookmarks an accessible post. Comments and non-post targets are rejected.
// @ID communitySetFavorite
// @Summary Bookmark or unbookmark a post
// @Tags Community
// @Accept json
// @Produce json
// @Param request body SetFavoriteRequest true "Favorite payload"
// @Success 200 {object} FavoriteResponse
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Failure 410 {object} auth.ErrorResponse
// @Router /api/community/favorites [put]
func (h *Handler) SetFavorite(c *gin.Context) {
	actor, ok := auth.RequireVerified(c, h.resolver)
	if !ok {
		return
	}

	var req SetFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	targetType := strings.TrimSpace(req.TargetType)
	if targetType != "" && targetType != "post" {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "favorite_post_only", Message: ErrFavoritePostOnly.Error()})
		return
	}

	postID := strings.TrimSpace(req.PostID)
	if postID == "" {
		postID = strings.TrimSpace(req.TargetID)
	}
	if postID == "" {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: "postId is required"})
		return
	}

	if h.postStore == nil {
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "internal_error", Message: "post store unavailable"})
		return
	}

	// Reject comment IDs explicitly with ErrFavoritePostOnly.
	if _, err := h.postStore.GetComment(c.Request.Context(), postID); err == nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "favorite_post_only", Message: ErrFavoritePostOnly.Error()})
		return
	}

	post, err := h.postStore.GetPost(c.Request.Context(), postID)
	if err != nil {
		c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "post_not_found", Message: "post not found"})
		return
	}

	if req.Saved {
		if post.Status == posts.StatusDeleted {
			c.JSON(http.StatusGone, auth.ErrorResponse{Error: "post_deleted", Message: "作者已删除，无法收藏"})
			return
		}
		if post.Status == posts.StatusHidden {
			c.JSON(http.StatusGone, auth.ErrorResponse{Error: "post_hidden", Message: "内容已隐藏，无法收藏"})
			return
		}
		if post.Status != posts.StatusPublished {
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "post_not_found", Message: "post unavailable"})
			return
		}
		if post.Scope == posts.ScopeCourse {
			allowed, err := h.verifyCourseAccess(c.Request.Context(), post.CourseID, actor.UserID)
			if err != nil {
				c.JSON(http.StatusServiceUnavailable, auth.ErrorResponse{Error: "course_access_unavailable", Message: "course access check unavailable"})
				return
			}
			if !allowed {
				c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "course_access_denied", Message: "current course access is required to bookmark course discussions"})
				return
			}
		}
	}

	saved, err := h.store.SetFavorite(c.Request.Context(), actor.UserID, postID, req.Saved)
	if err != nil {
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "favorite_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, FavoriteResponse{
		PostID: postID,
		Saved:  saved,
	})
}

// GetFavoriteStatus checks whether the current authenticated user has bookmarked a post.
// @ID communityGetFavoriteStatus
// @Summary Check if current user bookmarked a post
// @Tags Community
// @Produce json
// @Param postId path string true "Post ID"
// @Success 200 {object} FavoriteResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Router /api/community/favorites/{postId} [get]
func (h *Handler) GetFavoriteStatus(c *gin.Context) {
	actor, ok := auth.RequireAuthenticated(c, h.resolver)
	if !ok {
		return
	}

	postID := strings.TrimSpace(c.Param("postId"))
	saved, err := h.store.IsFavorited(c.Request.Context(), actor.UserID, postID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "favorite_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, FavoriteResponse{
		PostID: postID,
		Saved:  saved,
	})
}

// ListMyFavorites returns the current user's bookmarked post IDs.
// @ID communityListMyFavorites
// @Summary List current user bookmarked post IDs
// @Tags Community
// @Produce json
// @Success 200 {array} string
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Router /api/community/me/favorites [get]
func (h *Handler) ListMyFavorites(c *gin.Context) {
	actor, ok := auth.RequireAuthenticated(c, h.resolver)
	if !ok {
		return
	}

	postIDs, err := h.store.ListUserFavorites(c.Request.Context(), actor.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "favorite_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, postIDs)
}
