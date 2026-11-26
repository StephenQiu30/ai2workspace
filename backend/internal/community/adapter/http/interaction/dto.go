package interaction

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
