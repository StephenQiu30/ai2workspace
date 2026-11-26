package interaction

import (
	"context"
	"errors"
	"net/http"
	"strings"

	authhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/auth"
	interactionapp "github.com/StephenQiu30/ai2workspace/backend/internal/community/application/interaction"
	postsapp "github.com/StephenQiu30/ai2workspace/backend/internal/community/application/posts"
	interactiondomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/interaction"
	postsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/posts"
	"github.com/gin-gonic/gin"
)

// Handler manages interaction HTTP endpoints.
type Handler struct {
	store         interactionapp.Store
	postStore     postsapp.Store
	resolver      authhttp.SessionResolver
	courseChecker authhttp.CourseAccessChecker
}

// NewHandler creates an interaction Handler with trusted auth and target validation.
func NewHandler(
	store interactionapp.Store,
	postStore postsapp.Store,
	resolver authhttp.SessionResolver,
	courseChecker authhttp.CourseAccessChecker,
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
		c.JSON(http.StatusInternalServerError, authhttp.ErrorResponse{Error: "internal_error", Message: "post store unavailable"})
		return false
	}

	var post *postsdomain.Post
	switch targetType {
	case "post":
		p, err := h.postStore.GetPost(c.Request.Context(), targetID)
		if err != nil {
			c.JSON(http.StatusNotFound, authhttp.ErrorResponse{Error: "target_not_found", Message: "target post not found"})
			return false
		}
		if p.Status == postsdomain.StatusDeleted {
			c.JSON(http.StatusGone, authhttp.ErrorResponse{Error: "target_deleted", Message: "作者已删除，无法互动"})
			return false
		}
		if p.Status == postsdomain.StatusHidden {
			c.JSON(http.StatusGone, authhttp.ErrorResponse{Error: "target_hidden", Message: "内容已隐藏，无法互动"})
			return false
		}
		if p.Status != postsdomain.StatusPublished {
			c.JSON(http.StatusNotFound, authhttp.ErrorResponse{Error: "target_not_found", Message: "target post unavailable"})
			return false
		}
		post = p
	case "comment":
		cmt, err := h.postStore.GetComment(c.Request.Context(), targetID)
		if err != nil {
			c.JSON(http.StatusNotFound, authhttp.ErrorResponse{Error: "target_not_found", Message: "target comment not found"})
			return false
		}
		if cmt.Status == postsdomain.StatusDeleted || cmt.Status == postsdomain.StatusHidden || cmt.Status != postsdomain.StatusPublished {
			c.JSON(http.StatusGone, authhttp.ErrorResponse{Error: "target_unavailable", Message: "target comment is no longer available"})
			return false
		}
		p, err := h.postStore.GetPost(c.Request.Context(), cmt.PostID)
		if err != nil || p.Status != postsdomain.StatusPublished {
			c.JSON(http.StatusGone, authhttp.ErrorResponse{Error: "target_unavailable", Message: "parent post is no longer available"})
			return false
		}
		post = p
	default:
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "invalid_target_type", Message: interactiondomain.ErrInvalidTargetType.Error()})
		return false
	}

	if post.Scope == postsdomain.ScopeCourse {
		if userID == "" {
			c.JSON(http.StatusForbidden, authhttp.ErrorResponse{Error: "course_access_denied", Message: "current course access is required"})
			return false
		}
		allowed, err := h.verifyCourseAccess(c.Request.Context(), post.CourseID, userID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, authhttp.ErrorResponse{Error: "course_access_unavailable", Message: "course access check unavailable"})
			return false
		}
		if !allowed {
			c.JSON(http.StatusForbidden, authhttp.ErrorResponse{Error: "course_access_denied", Message: "current course access is required"})
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
// @Success 200 {object} interactiondomain.ReactionSummary
// @Failure 400 {object} authhttp.ErrorResponse
// @Failure 401 {object} authhttp.ErrorResponse
// @Failure 403 {object} authhttp.ErrorResponse
// @Failure 404 {object} authhttp.ErrorResponse
// @Failure 410 {object} authhttp.ErrorResponse
// @Router /api/community/reactions [put]
func (h *Handler) SetReaction(c *gin.Context) {
	actor, ok := authhttp.RequireVerified(c, h.resolver)
	if !ok {
		return
	}

	var req SetReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	targetType := strings.TrimSpace(req.TargetType)
	targetID := strings.TrimSpace(req.TargetID)
	val := interactiondomain.ReactionValue(strings.TrimSpace(req.Value))

	if targetType != "post" && targetType != "comment" {
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "invalid_target_type", Message: interactiondomain.ErrInvalidTargetType.Error()})
		return
	}
	if !interactiondomain.IsValidReactionValue(val) {
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "invalid_reaction_value", Message: interactiondomain.ErrInvalidReactionValue.Error()})
		return
	}

	if !h.validateTargetAccess(c, actor.UserID, targetType, targetID) {
		return
	}

	summary, err := h.store.SetReaction(c.Request.Context(), actor.UserID, targetType, targetID, val)
	if err != nil {
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "reaction_failed", Message: err.Error()})
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
// @Success 200 {object} interactiondomain.ReactionSummary
// @Failure 400 {object} authhttp.ErrorResponse
// @Failure 401 {object} authhttp.ErrorResponse
// @Failure 403 {object} authhttp.ErrorResponse
// @Failure 404 {object} authhttp.ErrorResponse
// @Failure 410 {object} authhttp.ErrorResponse
// @Router /api/community/reactions [get]
func (h *Handler) GetReaction(c *gin.Context) {
	targetType := strings.TrimSpace(c.Query("targetType"))
	targetID := strings.TrimSpace(c.Query("targetId"))
	if targetType != "post" && targetType != "comment" {
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "invalid_target_type", Message: interactiondomain.ErrInvalidTargetType.Error()})
		return
	}
	if targetID == "" {
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "invalid_request", Message: "targetId is required"})
		return
	}

	actor, authErr := authhttp.OptionalActor(c, h.resolver)
	if authErr != nil {
		if errors.Is(authErr, authhttp.ErrAccountSuspended) {
			authhttp.WriteAuthError(c, authErr)
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
		c.JSON(http.StatusUnauthorized, authhttp.ErrorResponse{Error: "unauthorized", Message: "authentication required to inspect comment reactions"})
		return
	}

	if !h.validateTargetAccess(c, viewerID, targetType, targetID) {
		return
	}

	summary, err := h.store.GetReaction(c.Request.Context(), viewerID, targetType, targetID)
	if err != nil {
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "reaction_failed", Message: err.Error()})
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
// @Failure 400 {object} authhttp.ErrorResponse
// @Failure 401 {object} authhttp.ErrorResponse
// @Failure 403 {object} authhttp.ErrorResponse
// @Failure 404 {object} authhttp.ErrorResponse
// @Failure 410 {object} authhttp.ErrorResponse
// @Router /api/community/favorites [put]
func (h *Handler) SetFavorite(c *gin.Context) {
	actor, ok := authhttp.RequireVerified(c, h.resolver)
	if !ok {
		return
	}

	var req SetFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	targetType := strings.TrimSpace(req.TargetType)
	if targetType != "" && targetType != "post" {
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "favorite_post_only", Message: interactiondomain.ErrFavoritePostOnly.Error()})
		return
	}

	postID := strings.TrimSpace(req.PostID)
	if postID == "" {
		postID = strings.TrimSpace(req.TargetID)
	}
	if postID == "" {
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "invalid_request", Message: "postId is required"})
		return
	}

	if h.postStore == nil {
		c.JSON(http.StatusInternalServerError, authhttp.ErrorResponse{Error: "internal_error", Message: "post store unavailable"})
		return
	}

	// Reject comment IDs explicitly with ErrFavoritePostOnly.
	if _, err := h.postStore.GetComment(c.Request.Context(), postID); err == nil {
		c.JSON(http.StatusBadRequest, authhttp.ErrorResponse{Error: "favorite_post_only", Message: interactiondomain.ErrFavoritePostOnly.Error()})
		return
	}

	post, err := h.postStore.GetPost(c.Request.Context(), postID)
	if err != nil {
		c.JSON(http.StatusNotFound, authhttp.ErrorResponse{Error: "post_not_found", Message: "post not found"})
		return
	}

	if req.Saved {
		if post.Status == postsdomain.StatusDeleted {
			c.JSON(http.StatusGone, authhttp.ErrorResponse{Error: "post_deleted", Message: "作者已删除，无法收藏"})
			return
		}
		if post.Status == postsdomain.StatusHidden {
			c.JSON(http.StatusGone, authhttp.ErrorResponse{Error: "post_hidden", Message: "内容已隐藏，无法收藏"})
			return
		}
		if post.Status != postsdomain.StatusPublished {
			c.JSON(http.StatusNotFound, authhttp.ErrorResponse{Error: "post_not_found", Message: "post unavailable"})
			return
		}
		if post.Scope == postsdomain.ScopeCourse {
			allowed, err := h.verifyCourseAccess(c.Request.Context(), post.CourseID, actor.UserID)
			if err != nil {
				c.JSON(http.StatusServiceUnavailable, authhttp.ErrorResponse{Error: "course_access_unavailable", Message: "course access check unavailable"})
				return
			}
			if !allowed {
				c.JSON(http.StatusForbidden, authhttp.ErrorResponse{Error: "course_access_denied", Message: "current course access is required to bookmark course discussions"})
				return
			}
		}
	}

	saved, err := h.store.SetFavorite(c.Request.Context(), actor.UserID, postID, req.Saved)
	if err != nil {
		c.JSON(http.StatusInternalServerError, authhttp.ErrorResponse{Error: "favorite_failed", Message: err.Error()})
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
// @Failure 401 {object} authhttp.ErrorResponse
// @Failure 403 {object} authhttp.ErrorResponse
// @Router /api/community/favorites/{postId} [get]
func (h *Handler) GetFavoriteStatus(c *gin.Context) {
	actor, ok := authhttp.RequireAuthenticated(c, h.resolver)
	if !ok {
		return
	}

	postID := strings.TrimSpace(c.Param("postId"))
	saved, err := h.store.IsFavorited(c.Request.Context(), actor.UserID, postID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, authhttp.ErrorResponse{Error: "favorite_failed", Message: err.Error()})
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
// @Failure 401 {object} authhttp.ErrorResponse
// @Failure 403 {object} authhttp.ErrorResponse
// @Router /api/community/me/favorites [get]
func (h *Handler) ListMyFavorites(c *gin.Context) {
	actor, ok := authhttp.RequireAuthenticated(c, h.resolver)
	if !ok {
		return
	}

	postIDs, err := h.store.ListUserFavorites(c.Request.Context(), actor.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, authhttp.ErrorResponse{Error: "favorite_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, postIDs)
}
