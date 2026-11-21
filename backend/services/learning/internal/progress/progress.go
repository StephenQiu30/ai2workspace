// Package progress manages student study position, progress isolation, and optimistic versioning.
package progress

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
	ErrProgressNotFound = errors.New("progress not found")
	ErrVersionConflict  = errors.New("optimistic version conflict: stale progress report")
	ErrInvalidPosition  = errors.New("position cannot be negative")
)

// Progress records a user's latest learning position in a lesson.
type Progress struct {
	ID              string    `json:"id"`
	UserID          string    `json:"userId"`
	CourseID        string    `json:"courseId"`
	LessonID        string    `json:"lessonId"`
	ContentVersion  int       `json:"contentVersion"`
	PositionSeconds int       `json:"positionSeconds"`
	Version         int       `json:"version"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// Store defines persistence operations for progress.
type Store interface {
	SaveProgress(ctx context.Context, userID, courseID, lessonID string, contentVersion, positionSeconds, expectedVersion int) (*Progress, error)
	GetProgress(ctx context.Context, userID, lessonID string) (*Progress, error)
	ListProgressByCourse(ctx context.Context, userID, courseID string) ([]*Progress, error)
}

// MemoryStore provides a thread-safe in-memory store for learning progress.
type MemoryStore struct {
	mu       sync.RWMutex
	progress map[string]*Progress // "userID:lessonID" -> Progress
}

// NewMemoryStore initializes a new MemoryStore for progress.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		progress: make(map[string]*Progress),
	}
}

func cloneProgress(p *Progress) *Progress {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

func generateProgressID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("prog_%s", hex.EncodeToString(b))
}

// SaveProgress creates or updates a user's learning progress using strict optimistic concurrency.
// expectedVersion == 0 is only valid for initial creation; updating an existing record requires matching its current version.
func (s *MemoryStore) SaveProgress(ctx context.Context, userID, courseID, lessonID string, contentVersion, positionSeconds, expectedVersion int) (*Progress, error) {
	if positionSeconds < 0 {
		return nil, ErrInvalidPosition
	}
	if expectedVersion < 0 {
		return nil, ErrVersionConflict
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s", userID, lessonID)
	existing, ok := s.progress[key]
	now := time.Now().UTC()

	if !ok {
		if expectedVersion != 0 {
			return nil, ErrVersionConflict
		}

		p := &Progress{
			ID:              generateProgressID(),
			UserID:          userID,
			CourseID:        courseID,
			LessonID:        lessonID,
			ContentVersion:  contentVersion,
			PositionSeconds: positionSeconds,
			Version:         1,
			UpdatedAt:       now,
		}
		s.progress[key] = p
		return cloneProgress(p), nil
	}

	if existing.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	existing.CourseID = courseID
	existing.ContentVersion = contentVersion
	existing.PositionSeconds = positionSeconds
	existing.Version++
	existing.UpdatedAt = now

	return cloneProgress(existing), nil
}

func (s *MemoryStore) GetProgress(ctx context.Context, userID, lessonID string) (*Progress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", userID, lessonID)
	p, ok := s.progress[key]
	if !ok {
		return nil, ErrProgressNotFound
	}
	return cloneProgress(p), nil
}

func (s *MemoryStore) ListProgressByCourse(ctx context.Context, userID, courseID string) ([]*Progress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*Progress
	for _, p := range s.progress {
		if p.UserID == userID && p.CourseID == courseID {
			list = append(list, cloneProgress(p))
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if !list[i].UpdatedAt.Equal(list[j].UpdatedAt) {
			return list[i].UpdatedAt.After(list[j].UpdatedAt)
		}
		return list[i].ID > list[j].ID
	})
	return list, nil
}
