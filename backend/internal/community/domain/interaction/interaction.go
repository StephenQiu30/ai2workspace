package interaction

import (
	"errors"
	"time"
)

var (
	// ErrInvalidTargetType indicates targetType is not post or comment.
	ErrInvalidTargetType = errors.New("targetType must be post or comment")
	// ErrInvalidReactionValue indicates value is not like, dislike, or none.
	ErrInvalidReactionValue = errors.New("reaction value must be like, dislike, or none")
	// ErrFavoritePostOnly indicates favorites only apply to posts, not comments or other targets.
	ErrFavoritePostOnly = errors.New("favorites are only supported for posts")
)

// ReactionValue represents a user's single mutually exclusive feedback state.
type ReactionValue string

const (
	ReactionLike    ReactionValue = "like"
	ReactionDislike ReactionValue = "dislike"
	ReactionNone    ReactionValue = "none"
)

// IsValidReactionValue checks whether v is one of like, dislike, or none.
func IsValidReactionValue(v ReactionValue) bool {
	return v == ReactionLike || v == ReactionDislike || v == ReactionNone
}

// Reaction records a user's single feedback state on a post or comment.
type Reaction struct {
	ID         string        `json:"id"`
	UserID     string        `json:"userId"`
	TargetType string        `json:"targetType"`
	TargetID   string        `json:"targetId"`
	Value      ReactionValue `json:"value"`
	UpdatedAt  time.Time     `json:"updatedAt"`
}

// Favorite records a user's bookmark of a post.
type Favorite struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	PostID    string    `json:"postId"`
	CreatedAt time.Time `json:"createdAt"`
}

// ReactionSummary aggregates counts and the current user's reaction state.
type ReactionSummary struct {
	TargetType string        `json:"targetType" example:"post"`
	TargetID   string        `json:"targetId" example:"post_12345"`
	Likes      int           `json:"likes" example:"12"`
	Dislikes   int           `json:"dislikes" example:"1"`
	MyReaction ReactionValue `json:"myReaction" example:"like"`
}
