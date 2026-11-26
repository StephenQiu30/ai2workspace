package moderation

import (
	"errors"
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
