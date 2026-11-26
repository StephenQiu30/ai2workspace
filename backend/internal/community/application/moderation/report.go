package moderation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	moderationdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/moderation"
)

// Store defines persistence operations for moderation reports.
type Store interface {
	CreateReport(ctx context.Context, reporterID, targetType, targetID, reason string) (*moderationdomain.Report, error)
	GetReport(ctx context.Context, reportID string) (*moderationdomain.Report, error)
	ResolveReport(ctx context.Context, reviewerID, reportID string, status moderationdomain.ReportStatus, reviewReason string) (*moderationdomain.Report, error)
	ListReports(ctx context.Context, statusFilter string, page, pageSize int) ([]*moderationdomain.Report, int, error)
}

// MemoryStore provides a thread-safe in-memory store for moderation reports.
type MemoryStore struct {
	mu            sync.RWMutex
	reports       map[string]*moderationdomain.Report
	lastTimestamp time.Time
}

// NewMemoryStore initializes a new MemoryStore for moderation.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		reports: make(map[string]*moderationdomain.Report),
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

func cloneReport(r *moderationdomain.Report) *moderationdomain.Report {
	if r == nil {
		return nil
	}
	cp := *r
	if r.ReviewedAt != nil {
		t := *r.ReviewedAt
		cp.ReviewedAt = &t
	}
	return &cp
}

// CreateReport creates a pending report, deduplicating open reports by the same reporter for the same target.
func (s *MemoryStore) CreateReport(ctx context.Context, reporterID, targetType, targetID, reason string) (*moderationdomain.Report, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, moderationdomain.ErrInvalidTargetType
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.reports {
		if existing.Status == moderationdomain.StatusPending &&
			existing.ReporterID == reporterID &&
			existing.TargetType == targetType &&
			existing.TargetID == targetID {
			return cloneReport(existing), nil
		}
	}

	now := s.nextTimestampLocked()
	reportID := NewID("rep")
	rep := &moderationdomain.Report{
		ID:         reportID,
		ReporterID: reporterID,
		TargetType: targetType,
		TargetID:   targetID,
		Reason:     strings.TrimSpace(reason),
		Status:     moderationdomain.StatusPending,
		CreatedAt:  now,
	}

	s.reports[reportID] = rep
	return cloneReport(rep), nil
}

// GetReport retrieves a report by ID.
func (s *MemoryStore) GetReport(ctx context.Context, reportID string) (*moderationdomain.Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rep, ok := s.reports[reportID]
	if !ok {
		return nil, moderationdomain.ErrReportNotFound
	}
	return cloneReport(rep), nil
}

// ResolveReport marks a pending report as approved or rejected.
func (s *MemoryStore) ResolveReport(ctx context.Context, reviewerID, reportID string, status moderationdomain.ReportStatus, reviewReason string) (*moderationdomain.Report, error) {
	if status != moderationdomain.StatusApproved && status != moderationdomain.StatusRejected {
		return nil, moderationdomain.ErrInvalidDecision
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rep, ok := s.reports[reportID]
	if !ok {
		return nil, moderationdomain.ErrReportNotFound
	}
	if rep.Status != moderationdomain.StatusPending {
		return nil, moderationdomain.ErrReportResolved
	}

	now := s.nextTimestampLocked()
	rep.ReviewerID = reviewerID
	rep.ReviewReason = strings.TrimSpace(reviewReason)
	rep.Status = status
	rep.ReviewedAt = &now
	return cloneReport(rep), nil
}

// ListReports returns reports ordered deterministically by CreatedAt DESC, ID DESC.
func (s *MemoryStore) ListReports(ctx context.Context, statusFilter string, page, pageSize int) ([]*moderationdomain.Report, int, error) {
	statusFilter = strings.TrimSpace(statusFilter)
	if statusFilter != "" &&
		statusFilter != string(moderationdomain.StatusPending) &&
		statusFilter != string(moderationdomain.StatusApproved) &&
		statusFilter != string(moderationdomain.StatusRejected) {
		return nil, 0, moderationdomain.ErrInvalidStatusFilter
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*moderationdomain.Report
	for _, r := range s.reports {
		if statusFilter != "" && string(r.Status) != statusFilter {
			continue
		}
		list = append(list, cloneReport(r))
	}

	sort.Slice(list, func(i, j int) bool {
		if !list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].CreatedAt.After(list[j].CreatedAt)
		}
		return list[i].ID > list[j].ID
	})

	total := len(list)
	start := (page - 1) * pageSize
	if start >= total {
		return []*moderationdomain.Report{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return list[start:end], total, nil
}
