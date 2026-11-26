package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	notificationapp "github.com/StephenQiu30/ai2workspace/backend/internal/community/application/notification"
	notificationdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/notification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists notifications in PostgreSQL using pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a PostgreSQL-backed notification store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// AppendNotification inserts a notification with durable deduplication on (event_id, recipient_id).
func (s *PostgresStore) AppendNotification(ctx context.Context, eventID, recipientID, notifType, title, summary, targetType, targetID string) (*notificationdomain.Notification, error) {
	notifID := notificationapp.NewID("notif")
	now := time.Now().UTC()

	var n notificationdomain.Notification
	err := s.pool.QueryRow(ctx, `
		INSERT INTO notifications (
			id, event_id, recipient_id, type, title, summary, target_type, target_id, read, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, FALSE, $9)
		ON CONFLICT (event_id, recipient_id)
		DO UPDATE SET event_id = notifications.event_id
		RETURNING id, event_id, recipient_id, type, title, summary, target_type, target_id, read, created_at
	`, notifID, eventID, recipientID, notifType, title, summary,
		strings.TrimSpace(targetType), strings.TrimSpace(targetID), now,
	).Scan(
		&n.ID, &n.EventID, &n.RecipientID, &n.Type, &n.Title,
		&n.Summary, &n.TargetType, &n.TargetID, &n.Read, &n.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("append notification: %w", err)
	}
	n.TargetAccessible = true
	return &n, nil
}

// ListNotifications returns paginated notifications for recipientID ordered by created_at DESC, id DESC.
func (s *PostgresStore) ListNotifications(ctx context.Context, recipientID string, page, pageSize int) ([]*notificationdomain.Notification, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM notifications WHERE recipient_id = $1
	`, recipientID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := s.pool.Query(ctx, `
		SELECT id, event_id, recipient_id, type, title, summary, target_type, target_id, read, created_at
		FROM notifications
		WHERE recipient_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, recipientID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	items := make([]*notificationdomain.Notification, 0)
	for rows.Next() {
		var n notificationdomain.Notification
		if err := rows.Scan(
			&n.ID, &n.EventID, &n.RecipientID, &n.Type, &n.Title,
			&n.Summary, &n.TargetType, &n.TargetID, &n.Read, &n.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		n.TargetAccessible = true
		items = append(items, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate notifications: %w", err)
	}
	return items, total, nil
}

// MarkRead marks a recipient's notification as read after checking ownership.
func (s *PostgresStore) MarkRead(ctx context.Context, recipientID, notificationID string) error {
	var ownerID string
	err := s.pool.QueryRow(ctx, `
		SELECT recipient_id FROM notifications WHERE id = $1
	`, notificationID).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notificationdomain.ErrNotificationNotFound
		}
		return fmt.Errorf("lookup notification: %w", err)
	}
	if ownerID != recipientID {
		return notificationdomain.ErrPermissionDenied
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE notifications SET read = TRUE WHERE id = $1 AND recipient_id = $2
	`, notificationID, recipientID); err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	return nil
}
