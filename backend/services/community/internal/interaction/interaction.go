// Package interaction manages mutually exclusive like/dislike reactions and post bookmark favorites.
package interaction

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
	ErrInvalidTargetType = errors.New("target type must be post or comment")
	ErrFavoritePostOnly  = errors.New("favorites are only supported for posts")
)

// ReactionValue represents single user choice.
type ReactionValue string

const (
	ReactionLike    ReactionValue = "like"
	ReactionDislike ReactionValue = "dislike"
	ReactionNone    ReactionValue = "none"
)

// Reaction records a user's single feedback state on a post or comment.
type Reaction struct {
	ID         string        `json:"id"`
	UserID     string        `json:"userId"`
	TargetType string        `json:"targetType"` // "post", "comment"
	TargetID   string        `json:"targetId"`
	Value      ReactionValue `json:"value"`
	UpdatedAt  time.Time     `json:"updatedAt"`
}

// Favorite records user bookmarking of a post.
type Favorite struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	PostID    string    `json:"postId"`
	CreatedAt time.Time `json:"createdAt"`
}

// ReactionSummary aggregates counts and current user's reaction.
type ReactionSummary struct {
	TargetType string        `json:"targetType"`
	TargetID   string        `json:"targetId"`
	Likes      int           `json:"likes"`
	Dislikes   int           `json:"dislikes"`
	MyReaction ReactionValue `json:"myReaction"`
}

// Store defines persistence operations for reactions and favorites.
type Store interface {
	SetReaction(ctx context.Context, userID, targetType, targetID string, value ReactionValue) (*ReactionSummary, error)
	GetReaction(ctx context.Context, userID, targetType, targetID string) (*ReactionSummary, error)
	SetFavorite(ctx context.Context, userID, postID string, saved bool) (bool, error)
	ListUserFavorites(ctx context.Context, userID string) ([]string, error)
}

// MemoryStore provides in-memory thread-safe store for interactions.
type MemoryStore struct {
	mu        sync.RWMutex
	reactions map[string]*Reaction // "userID:targetType:targetID" -> Reaction
	favorites map[string]*Favorite // "userID:postID" -> Favorite
}

// NewMemoryStore initializes a new MemoryStore for interaction.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		reactions: make(map[string]*Reaction),
		favorites: make(map[string]*Favorite),
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (s *MemoryStore) SetReaction(ctx context.Context, userID, targetType, targetID string, value ReactionValue) (*ReactionSummary, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, ErrInvalidTargetType
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%s", userID, targetType, targetID)
	now := time.Now()

	if value == ReactionNone {
		delete(s.reactions, key)
	} else {
		existing, ok := s.reactions[key]
		if ok {
			existing.Value = value
			existing.UpdatedAt = now
		} else {
			s.reactions[key] = &Reaction{
				ID:         generateID("rx"),
				UserID:     userID,
				TargetType: targetType,
				TargetID:   targetID,
				Value:      value,
				UpdatedAt:  now,
			}
		}
	}

	// Calculate counts across all users for this target
	likes := 0
	dislikes := 0
	for _, r := range s.reactions {
		if r.TargetType == targetType && r.TargetID == targetID {
			if r.Value == ReactionLike {
				likes++
			} else if r.Value == ReactionDislike {
				dislikes++
			}
		}
	}

	return &ReactionSummary{
		TargetType: targetType,
		TargetID:   targetID,
		Likes:      likes,
		Dislikes:   dislikes,
		MyReaction: value,
	}, nil
}

func (s *MemoryStore) GetReaction(ctx context.Context, userID, targetType, targetID string) (*ReactionSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s:%s", userID, targetType, targetID)
	myReaction := ReactionNone
	if r, ok := s.reactions[key]; ok {
		myReaction = r.Value
	}

	likes := 0
	dislikes := 0
	for _, r := range s.reactions {
		if r.TargetType == targetType && r.TargetID == targetID {
			if r.Value == ReactionLike {
				likes++
			} else if r.Value == ReactionDislike {
				dislikes++
			}
		}
	}

	return &ReactionSummary{
		TargetType: targetType,
		TargetID:   targetID,
		Likes:      likes,
		Dislikes:   dislikes,
		MyReaction: myReaction,
	}, nil
}

func (s *MemoryStore) SetFavorite(ctx context.Context, userID, postID string, saved bool) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s", userID, postID)
	if saved {
		if _, exists := s.favorites[key]; !exists {
			s.favorites[key] = &Favorite{
				ID:        generateID("fav"),
				UserID:    userID,
				PostID:    postID,
				CreatedAt: time.Now(),
			}
		}
		return true, nil
	}

	delete(s.favorites, key)
	return false, nil
}

func (s *MemoryStore) ListUserFavorites(ctx context.Context, userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var postIDs []string
	for _, f := range s.favorites {
		if f.UserID == userID {
			postIDs = append(postIDs, f.PostID)
		}
	}
	return postIDs, nil
}
