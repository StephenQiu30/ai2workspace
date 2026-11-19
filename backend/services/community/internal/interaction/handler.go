package interaction

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SetReactionRequest defines payload to like, dislike, or cancel feedback.
type SetReactionRequest struct {
	TargetType string `json:"targetType" binding:"required"` // "post", "comment"
	TargetID   string `json:"targetId" binding:"required"`
	Value      string `json:"value" binding:"required"` // "like", "dislike", "none"
}

// SetFavoriteRequest defines payload to bookmark or unbookmark a post.
type SetFavoriteRequest struct {
	PostID string `json:"postId" binding:"required"`
	Saved  bool   `json:"saved"`
}

// FavoriteResponse represents bookmarking result.
type FavoriteResponse struct {
	PostID string `json:"postId"`
	Saved  bool   `json:"saved"`
}

// Handler manages interaction HTTP endpoints.
type Handler struct {
	store Store
}

// NewHandler creates an interaction Handler.
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// SetReaction sets or toggles like/dislike feedback.
// @ID communitySetReaction
// @Summary Set or toggle like/dislike reaction
// @Tags Community
// @Accept json
// @Produce json
// @Param request body SetReactionRequest true "Reaction payload"
// @Success 200 {object} ReactionSummary
// @Failure 400 {object} map[string]string
// @Router /api/community/reactions [post]
func (h *Handler) SetReaction(c *gin.Context) {
	var req SetReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "mock_user"
	}

	summary, err := h.store.SetReaction(c.Request.Context(), userID, req.TargetType, req.TargetID, ReactionValue(req.Value))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetReaction returns reaction counts and current user's reaction for a target.
// @ID communityGetReaction
// @Summary Get reaction summary for target
// @Tags Community
// @Produce json
// @Param targetType query string true "Target type: post or comment"
// @Param targetId query string true "Target ID"
// @Success 200 {object} ReactionSummary
// @Failure 400 {object} map[string]string
// @Router /api/community/reactions [get]
func (h *Handler) GetReaction(c *gin.Context) {
	targetType := c.Query("targetType")
	targetID := c.Query("targetId")
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "mock_user"
	}

	summary, err := h.store.GetReaction(c.Request.Context(), userID, targetType, targetID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// SetFavorite toggles post bookmarking for current user.
// @ID communitySetFavorite
// @Summary Bookmark or unbookmark post
// @Tags Community
// @Accept json
// @Produce json
// @Param request body SetFavoriteRequest true "Favorite payload"
// @Success 200 {object} FavoriteResponse
// @Failure 400 {object} map[string]string
// @Router /api/community/favorites [post]
func (h *Handler) SetFavorite(c *gin.Context) {
	var req SetFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "mock_user"
	}

	saved, err := h.store.SetFavorite(c.Request.Context(), userID, req.PostID, req.Saved)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, FavoriteResponse{
		PostID: req.PostID,
		Saved:  saved,
	})
}

// ListMyFavorites returns list of post IDs bookmarked by user.
// @ID communityListMyFavorites
// @Summary List current user bookmarked post IDs
// @Tags Community
// @Produce json
// @Success 200 {array} string
// @Router /api/community/me/favorites [get]
func (h *Handler) ListMyFavorites(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "mock_user"
	}

	postIDs, err := h.store.ListUserFavorites(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, postIDs)
}
