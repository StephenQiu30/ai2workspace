package moderation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists moderation reports in PostgreSQL using pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a PostgreSQL-backed moderation store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// CreateReport creates a pending report or returns an existing open report by the same reporter for the same target.
func (s *PostgresStore) CreateReport(ctx context.Context, reporterID, targetType, targetID, reason string) (*Report, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, ErrInvalidTargetType
	}

	var existing Report
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, reporter_id, target_type, target_id, reason, status, review_reason, reviewer_id, created_at, reviewed_at
		FROM reports
		WHERE reporter_id = $1 AND target_type = $2 AND target_id = $3 AND status = 'pending'
		LIMIT 1
	`, reporterID, targetType, targetID).Scan(
		&existing.ID, &existing.ReporterID, &existing.TargetType, &existing.TargetID,
		&existing.Reason, &statusStr, &existing.ReviewReason, &existing.ReviewerID,
		&existing.CreatedAt, &existing.ReviewedAt,
	)
	if err == nil {
		existing.Status = ReportStatus(statusStr)
		return &existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check existing report: %w", err)
	}

	reportID := generateID("rep")
	now := time.Now().UTC()
	var rep Report
	err = s.pool.QueryRow(ctx, `
		INSERT INTO reports (id, reporter_id, target_type, target_id, reason, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
		RETURNING id, reporter_id, target_type, target_id, reason, status, review_reason, reviewer_id, created_at, reviewed_at
	`, reportID, reporterID, targetType, targetID, strings.TrimSpace(reason), now).Scan(
		&rep.ID, &rep.ReporterID, &rep.TargetType, &rep.TargetID,
		&rep.Reason, &statusStr, &rep.ReviewReason, &rep.ReviewerID,
		&rep.CreatedAt, &rep.ReviewedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert report: %w", err)
	}
	rep.Status = ReportStatus(statusStr)
	return &rep, nil
}

// GetReport fetches a single report by ID.
func (s *PostgresStore) GetReport(ctx context.Context, reportID string) (*Report, error) {
	var rep Report
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, reporter_id, target_type, target_id, reason, status, review_reason, reviewer_id, created_at, reviewed_at
		FROM reports
		WHERE id = $1
	`, reportID).Scan(
		&rep.ID, &rep.ReporterID, &rep.TargetType, &rep.TargetID,
		&rep.Reason, &statusStr, &rep.ReviewReason, &rep.ReviewerID,
		&rep.CreatedAt, &rep.ReviewedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReportNotFound
		}
		return nil, fmt.Errorf("get report: %w", err)
	}
	rep.Status = ReportStatus(statusStr)
	return &rep, nil
}

// ResolveReport marks a pending report as approved or rejected.
func (s *PostgresStore) ResolveReport(ctx context.Context, reviewerID, reportID string, status ReportStatus, reviewReason string) (*Report, error) {
	if status != StatusApproved && status != StatusRejected {
		return nil, ErrInvalidDecision
	}

	now := time.Now().UTC()
	var rep Report
	var statusStr string
	err := s.pool.QueryRow(ctx, `
		UPDATE reports
		SET status = $1, reviewer_id = $2, review_reason = $3, reviewed_at = $4
		WHERE id = $5 AND status = 'pending'
		RETURNING id, reporter_id, target_type, target_id, reason, status, review_reason, reviewer_id, created_at, reviewed_at
	`, string(status), reviewerID, strings.TrimSpace(reviewReason), now, reportID).Scan(
		&rep.ID, &rep.ReporterID, &rep.TargetType, &rep.TargetID,
		&rep.Reason, &statusStr, &rep.ReviewReason, &rep.ReviewerID,
		&rep.CreatedAt, &rep.ReviewedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, lookupErr := s.GetReport(ctx, reportID); lookupErr != nil {
				return nil, lookupErr
			}
			return nil, ErrReportResolved
		}
		return nil, fmt.Errorf("resolve report: %w", err)
	}
	rep.Status = ReportStatus(statusStr)
	return &rep, nil
}

// ListReports lists reports ordered by created_at DESC, id DESC.
func (s *PostgresStore) ListReports(ctx context.Context, statusFilter string, page, pageSize int) ([]*Report, int, error) {
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

	whereClause := ""
	args := []any{}
	argPos := 1
	if statusFilter != "" {
		whereClause = "WHERE status = $1"
		args = append(args, statusFilter)
		argPos = 2
	}

	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM reports "+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`
		SELECT id, reporter_id, target_type, target_id, reason, status, review_reason, reviewer_id, created_at, reviewed_at
		FROM reports
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argPos, argPos+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	items := make([]*Report, 0)
	for rows.Next() {
		var rep Report
		var statusStr string
		if err := rows.Scan(
			&rep.ID, &rep.ReporterID, &rep.TargetType, &rep.TargetID,
			&rep.Reason, &statusStr, &rep.ReviewReason, &rep.ReviewerID,
			&rep.CreatedAt, &rep.ReviewedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan report: %w", err)
		}
		rep.Status = ReportStatus(statusStr)
		items = append(items, &rep)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate reports: %w", err)
	}
	return items, total, nil
}
