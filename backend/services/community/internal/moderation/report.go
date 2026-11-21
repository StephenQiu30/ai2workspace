// Package moderation manages community report queues, auditing, and content hiding.
package moderation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// ErrReportNotFound indicates the requested report does not exist.
	ErrReportNotFound = errors.New("report not found")
	// ErrReportResolved indicates the report has already been reviewed.
	ErrReportResolved = errors.New("report has already been reviewed")
	// ErrInvalidTargetType indicates targetType must be post or comment.
	ErrInvalidTargetType = errors.New("targetType must be post or comment")
	// ErrInvalidDecision indicates decision must be approved or rejected.
	ErrInvalidDecision = errors.New("decision must be approved or rejected")
	// ErrInvalidStatusFilter indicates an unrecognized report status filter.
	ErrInvalidStatusFilter = errors.New("status must be pending, approved, or rejected")
)

// ReportStatus defines the moderation workflow state of a report.
type ReportStatus string

const (
	StatusPending  ReportStatus = "pending"
	StatusApproved ReportStatus = "approved"
	StatusRejected ReportStatus = "rejected"
)

// Report represents a user flag against a post or comment.
type Report struct {
	ID           string       `json:"id" example:"rep_12345"`
	ReporterID   string       `json:"reporterId" example:"usr_12345"`
	TargetType   string       `json:"targetType" example:"post"`
	TargetID     string       `json:"targetId" example:"post_12345"`
	Reason       string       `json:"reason" example:"广告垃圾信息"`
	Status       ReportStatus `json:"status" example:"pending"`
	ReviewReason string       `json:"reviewReason,omitempty" example:"核实属违规推广，予以隐藏"`
	ReviewerID   string       `json:"reviewerId,omitempty" example:"usr_admin"`
	CreatedAt    time.Time    `json:"createdAt"`
	ReviewedAt   *time.Time   `json:"reviewedAt,omitempty"`
}

// Store defines persistence operations for moderation reports.
type Store interface {
	CreateReport(ctx context.Context, reporterID, targetType, targetID, reason string) (*Report, error)
	GetReport(ctx context.Context, reportID string) (*Report, error)
	ResolveReport(ctx context.Context, reviewerID, reportID string, status ReportStatus, reviewReason string) (*Report, error)
	ListReports(ctx context.Context, statusFilter string, page, pageSize int) ([]*Report, int, error)
}

// MemoryStore provides a thread-safe in-memory store for moderation reports.
type MemoryStore struct {
	mu            sync.RWMutex
	reports       map[string]*Report
	lastTimestamp time.Time
}

// NewMemoryStore initializes a new MemoryStore for moderation.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		reports: make(map[string]*Report),
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

func cloneReport(r *Report) *Report {
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
func (s *MemoryStore) CreateReport(ctx context.Context, reporterID, targetType, targetID, reason string) (*Report, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, ErrInvalidTargetType
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.reports {
		if existing.Status == StatusPending &&
			existing.ReporterID == reporterID &&
			existing.TargetType == targetType &&
			existing.TargetID == targetID {
			return cloneReport(existing), nil
		}
	}

	now := s.nextTimestampLocked()
	reportID := generateID("rep")
	rep := &Report{
		ID:         reportID,
		ReporterID: reporterID,
		TargetType: targetType,
		TargetID:   targetID,
		Reason:     strings.TrimSpace(reason),
		Status:     StatusPending,
		CreatedAt:  now,
	}

	s.reports[reportID] = rep
	return cloneReport(rep), nil
}

// GetReport retrieves a report by ID.
func (s *MemoryStore) GetReport(ctx context.Context, reportID string) (*Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rep, ok := s.reports[reportID]
	if !ok {
		return nil, ErrReportNotFound
	}
	return cloneReport(rep), nil
}

// ResolveReport marks a pending report as approved or rejected.
func (s *MemoryStore) ResolveReport(ctx context.Context, reviewerID, reportID string, status ReportStatus, reviewReason string) (*Report, error) {
	if status != StatusApproved && status != StatusRejected {
		return nil, ErrInvalidDecision
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rep, ok := s.reports[reportID]
	if !ok {
		return nil, ErrReportNotFound
	}
	if rep.Status != StatusPending {
		return nil, ErrReportResolved
	}

	now := s.nextTimestampLocked()
	rep.ReviewerID = reviewerID
	rep.ReviewReason = strings.TrimSpace(reviewReason)
	rep.Status = status
	rep.ReviewedAt = &now
	return cloneReport(rep), nil
}

// ListReports returns reports ordered deterministically by CreatedAt DESC, ID DESC.
func (s *MemoryStore) ListReports(ctx context.Context, statusFilter string, page, pageSize int) ([]*Report, int, error) {
	statusFilter = strings.TrimSpace(statusFilter)
	if statusFilter != "" &&
		statusFilter != string(StatusPending) &&
		statusFilter != string(StatusApproved) &&
		statusFilter != string(StatusRejected) {
		return nil, 0, ErrInvalidStatusFilter
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*Report
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
		return []*Report{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return list[start:end], total, nil
}
