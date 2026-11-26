package interaction

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	interactiondomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/interaction"
)

// Store defines persistence operations for reactions and favorites.
type Store interface {
	SetReaction(ctx context.Context, userID, targetType, targetID string, value interactiondomain.ReactionValue) (*interactiondomain.ReactionSummary, error)
	GetReaction(ctx context.Context, userID, targetType, targetID string) (*interactiondomain.ReactionSummary, error)
	SetFavorite(ctx context.Context, userID, postID string, saved bool) (bool, error)
	IsFavorited(ctx context.Context, userID, postID string) (bool, error)
	ListUserFavorites(ctx context.Context, userID string) ([]string, error)
}

// MemoryStore provides a thread-safe in-memory store for reactions and favorites.
type MemoryStore struct {
	mu            sync.RWMutex
	reactions     map[string]*interactiondomain.Reaction // "userID:targetType:targetID" -> Reaction
	favorites     map[string]*interactiondomain.Favorite // "userID:postID" -> Favorite
	lastTimestamp time.Time
}

// NewMemoryStore initializes a new MemoryStore for interactions.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		reactions: make(map[string]*interactiondomain.Reaction),
		favorites: make(map[string]*interactiondomain.Favorite),
	}
}

func NewID(prefix string) string {
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
func (s *MemoryStore) SetReaction(ctx context.Context, userID, targetType, targetID string, value interactiondomain.ReactionValue) (*interactiondomain.ReactionSummary, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, interactiondomain.ErrInvalidTargetType
	}
	if !interactiondomain.IsValidReactionValue(value) {
		return nil, interactiondomain.ErrInvalidReactionValue
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%s", userID, targetType, targetID)
	now := s.nextTimestampLocked()

	if value == interactiondomain.ReactionNone {
		delete(s.reactions, key)
	} else {
		existing, ok := s.reactions[key]
		if ok {
			existing.Value = value
			existing.UpdatedAt = now
		} else {
			s.reactions[key] = &interactiondomain.Reaction{
				ID:         NewID("rx"),
				UserID:     userID,
				TargetType: targetType,
				TargetID:   targetID,
				Value:      value,
				UpdatedAt:  now,
			}
		}
	}

	likes, dislikes := s.countTargetLocked(targetType, targetID)
	return &interactiondomain.ReactionSummary{
		TargetType: targetType,
		TargetID:   targetID,
		Likes:      likes,
		Dislikes:   dislikes,
		MyReaction: value,
	}, nil
}

// GetReaction returns aggregated counts and the viewer's reaction state.
func (s *MemoryStore) GetReaction(ctx context.Context, userID, targetType, targetID string) (*interactiondomain.ReactionSummary, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, interactiondomain.ErrInvalidTargetType
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	myReaction := interactiondomain.ReactionNone
	if userID != "" {
		key := fmt.Sprintf("%s:%s:%s", userID, targetType, targetID)
		if r, ok := s.reactions[key]; ok {
			myReaction = r.Value
		}
	}

	likes, dislikes := s.countTargetLocked(targetType, targetID)
	return &interactiondomain.ReactionSummary{
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
			if r.Value == interactiondomain.ReactionLike {
				likes++
			} else if r.Value == interactiondomain.ReactionDislike {
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
			s.favorites[key] = &interactiondomain.Favorite{
				ID:        NewID("fav"),
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

	var favs []*interactiondomain.Favorite
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
