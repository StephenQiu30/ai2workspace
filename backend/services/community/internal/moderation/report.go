// Package moderation manages community report queues, auditing, and content hiding.
package moderation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/posts"
)

var (
	ErrReportNotFound = errors.New("report not found")
	ErrReportResolved = errors.New("report has already been reviewed")
)

// ReportStatus defines moderation state.
type ReportStatus string

const (
	StatusPending  ReportStatus = "pending"
	StatusApproved ReportStatus = "approved"
	StatusRejected ReportStatus = "rejected"
)

// Report represents a user flag against violation content.
type Report struct {
	ID           string       `json:"id"`
	ReporterID   string       `json:"reporterId"`
	TargetType   string       `json:"targetType"` // "post", "comment"
	TargetID     string       `json:"targetId"`
	Reason       string       `json:"reason"`
	Status       ReportStatus `json:"status"`
	ReviewReason string       `json:"reviewReason,omitempty"`
	ReviewerID   string       `json:"reviewerId,omitempty"`
	CreatedAt    time.Time    `json:"createdAt"`
	ReviewedAt   *time.Time   `json:"reviewedAt,omitempty"`
}

// Store defines persistence operations for moderation reports.
type Store interface {
	CreateReport(ctx context.Context, reporterID, targetType, targetID, reason string) (*Report, error)
	ModerateReport(ctx context.Context, reviewerID, reportID string, approved bool, reviewReason string) (*Report, error)
	ListReports(ctx context.Context, page, limit int) ([]*Report, int, error)
}

// MemoryStore provides in-memory store for moderation.
type MemoryStore struct {
	mu        sync.RWMutex
	reports   map[string]*Report
	postStore posts.Store
}

// NewMemoryStore initializes a new MemoryStore for moderation.
func NewMemoryStore(postStore posts.Store) *MemoryStore {
	return &MemoryStore{
		reports:   make(map[string]*Report),
		postStore: postStore,
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (s *MemoryStore) CreateReport(ctx context.Context, reporterID, targetType, targetID, reason string) (*Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	reportID := generateID("rep")
	rep := &Report{
		ID:         reportID,
		ReporterID: reporterID,
		TargetType: targetType,
		TargetID:   targetID,
		Reason:     reason,
		Status:     StatusPending,
		CreatedAt:  now,
	}

	s.reports[reportID] = rep
	return rep, nil
}

func (s *MemoryStore) ModerateReport(ctx context.Context, reviewerID, reportID string, approved bool, reviewReason string) (*Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rep, ok := s.reports[reportID]
	if !ok {
		return nil, ErrReportNotFound
	}
	if rep.Status != StatusPending {
		return nil, ErrReportResolved
	}

	now := time.Now()
	rep.ReviewerID = reviewerID
	rep.ReviewReason = reviewReason
	rep.ReviewedAt = &now

	if approved {
		rep.Status = StatusApproved
		// Atomically hide the target violation content
		if s.postStore != nil {
			if rep.TargetType == "post" {
				_ = s.postStore.HidePost(ctx, rep.TargetID)
			} else if rep.TargetType == "comment" {
				_ = s.postStore.HideComment(ctx, rep.TargetID)
			}
		}
	} else {
		rep.Status = StatusRejected
	}

	return rep, nil
}

func (s *MemoryStore) ListReports(ctx context.Context, page, limit int) ([]*Report, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var list []*Report
	for _, r := range s.reports {
		list = append(list, r)
	}

	total := len(list)
	start := (page - 1) * limit
	if start >= total {
		return []*Report{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	return list[start:end], total, nil
}
