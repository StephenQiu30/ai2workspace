package progress

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	progressdomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/progress"
)

// Store defines persistence operations for progress.
type Store interface {
	SaveProgress(ctx context.Context, userID, courseID, lessonID string, contentVersion, positionSeconds, expectedVersion int) (*progressdomain.Progress, error)
	GetProgress(ctx context.Context, userID, lessonID string) (*progressdomain.Progress, error)
	ListProgressByCourse(ctx context.Context, userID, courseID string) ([]*progressdomain.Progress, error)
}

// MemoryStore provides a thread-safe in-memory store for learning progress.
type MemoryStore struct {
	mu       sync.RWMutex
	progress map[string]*progressdomain.Progress // "userID:lessonID" -> Progress
}

// NewMemoryStore initializes a new MemoryStore for progress.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		progress: make(map[string]*progressdomain.Progress),
	}
}

func cloneProgress(p *progressdomain.Progress) *progressdomain.Progress {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

func GenerateProgressID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("prog_%s", hex.EncodeToString(b))
}

// SaveProgress creates or updates a user's learning progress using strict optimistic concurrency.
// expectedVersion == 0 is only valid for initial creation; updating an existing record requires matching its current version.
func (s *MemoryStore) SaveProgress(ctx context.Context, userID, courseID, lessonID string, contentVersion, positionSeconds, expectedVersion int) (*progressdomain.Progress, error) {
	if positionSeconds < 0 {
		return nil, progressdomain.ErrInvalidPosition
	}
	if expectedVersion < 0 {
		return nil, progressdomain.ErrVersionConflict
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s", userID, lessonID)
	existing, ok := s.progress[key]
	now := time.Now().UTC()

	if !ok {
		if expectedVersion != 0 {
			return nil, progressdomain.ErrVersionConflict
		}

		p := &progressdomain.Progress{
			ID:              GenerateProgressID(),
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
		return nil, progressdomain.ErrVersionConflict
	}

	existing.CourseID = courseID
	existing.ContentVersion = contentVersion
	existing.PositionSeconds = positionSeconds
	existing.Version++
	existing.UpdatedAt = now

	return cloneProgress(existing), nil
}

func (s *MemoryStore) GetProgress(ctx context.Context, userID, lessonID string) (*progressdomain.Progress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", userID, lessonID)
	p, ok := s.progress[key]
	if !ok {
		return nil, progressdomain.ErrProgressNotFound
	}
	return cloneProgress(p), nil
}

func (s *MemoryStore) ListProgressByCourse(ctx context.Context, userID, courseID string) ([]*progressdomain.Progress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*progressdomain.Progress
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
