// Package interaction manages mutually exclusive like/dislike reactions and post-only bookmark favorites.
package interaction

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
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

// Store defines persistence operations for reactions and favorites.
type Store interface {
	SetReaction(ctx context.Context, userID, targetType, targetID string, value ReactionValue) (*ReactionSummary, error)
	GetReaction(ctx context.Context, userID, targetType, targetID string) (*ReactionSummary, error)
	SetFavorite(ctx context.Context, userID, postID string, saved bool) (bool, error)
	IsFavorited(ctx context.Context, userID, postID string) (bool, error)
	ListUserFavorites(ctx context.Context, userID string) ([]string, error)
}

// MemoryStore provides a thread-safe in-memory store for reactions and favorites.
type MemoryStore struct {
	mu            sync.RWMutex
	reactions     map[string]*Reaction // "userID:targetType:targetID" -> Reaction
	favorites     map[string]*Favorite // "userID:postID" -> Favorite
	lastTimestamp time.Time
}

// NewMemoryStore initializes a new MemoryStore for interactions.
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

func (s *MemoryStore) nextTimestampLocked() time.Time {
	now := time.Now().UTC()
	if !now.After(s.lastTimestamp) {
		now = s.lastTimestamp.Add(time.Microsecond)
	}
	s.lastTimestamp = now
	return now
}

// SetReaction atomically updates a user's single reaction state and returns recalculated counts.
func (s *MemoryStore) SetReaction(ctx context.Context, userID, targetType, targetID string, value ReactionValue) (*ReactionSummary, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, ErrInvalidTargetType
	}
	if !IsValidReactionValue(value) {
		return nil, ErrInvalidReactionValue
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%s", userID, targetType, targetID)
	now := s.nextTimestampLocked()

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

	likes, dislikes := s.countTargetLocked(targetType, targetID)
	return &ReactionSummary{
		TargetType: targetType,
		TargetID:   targetID,
		Likes:      likes,
		Dislikes:   dislikes,
		MyReaction: value,
	}, nil
}

// GetReaction returns aggregated counts and the viewer's reaction state.
func (s *MemoryStore) GetReaction(ctx context.Context, userID, targetType, targetID string) (*ReactionSummary, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, ErrInvalidTargetType
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	myReaction := ReactionNone
	if userID != "" {
		key := fmt.Sprintf("%s:%s:%s", userID, targetType, targetID)
		if r, ok := s.reactions[key]; ok {
			myReaction = r.Value
		}
	}

	likes, dislikes := s.countTargetLocked(targetType, targetID)
	return &ReactionSummary{
		TargetType: targetType,
		TargetID:   targetID,
		Likes:      likes,
		Dislikes:   dislikes,
		MyReaction: myReaction,
	}, nil
}

func (s *MemoryStore) countTargetLocked(targetType, targetID string) (int, int) {
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
	return likes, dislikes
}

// SetFavorite bookmarks or unbookmarks a post for a user.
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
				CreatedAt: s.nextTimestampLocked(),
			}
		}
		return true, nil
	}

	delete(s.favorites, key)
	return false, nil
}

// IsFavorited returns true if userID currently has postID bookmarked.
func (s *MemoryStore) IsFavorited(ctx context.Context, userID, postID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", userID, postID)
	_, exists := s.favorites[key]
	return exists, nil
}

// ListUserFavorites returns the user's bookmarked post IDs sorted by bookmark creation time DESC.
func (s *MemoryStore) ListUserFavorites(ctx context.Context, userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var favs []*Favorite
	for _, f := range s.favorites {
		if f.UserID == userID {
			favs = append(favs, f)
		}
	}

	sort.Slice(favs, func(i, j int) bool {
		if !favs[i].CreatedAt.Equal(favs[j].CreatedAt) {
			return favs[i].CreatedAt.After(favs[j].CreatedAt)
		}
		return favs[i].PostID > favs[j].PostID
	})

	postIDs := make([]string, 0, len(favs))
	for _, f := range favs {
		postIDs = append(postIDs, f.PostID)
	}
	return postIDs, nil
}
