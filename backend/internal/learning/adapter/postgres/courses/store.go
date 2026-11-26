package courses

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	coursesapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/courses"
	coursesdomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/courses"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store backed by PostgreSQL using pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore initializes a PostgreSQL-backed courses Store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) loadSectionsAndLessons(ctx context.Context, courseID string) ([]coursesdomain.Section, int, error) {
	secRows, err := s.pool.Query(ctx, `
		SELECT id, course_id, title, sort_order
		FROM sections
		WHERE course_id = $1
		ORDER BY sort_order ASC, created_at ASC, id ASC
	`, courseID)
	if err != nil {
		return nil, 0, err
	}
	defer secRows.Close()

	sections := make([]coursesdomain.Section, 0)
	secIndex := make(map[string]int)
	for secRows.Next() {
		var sec coursesdomain.Section
		if err := secRows.Scan(&sec.ID, &sec.CourseID, &sec.Title, &sec.SortOrder); err != nil {
			return nil, 0, err
		}
		sec.Lessons = make([]coursesdomain.Lesson, 0)
		secIndex[sec.ID] = len(sections)
		sections = append(sections, sec)
	}
	if err := secRows.Err(); err != nil {
		return nil, 0, err
	}

	lsnRows, err := s.pool.Query(ctx, `
		SELECT id, section_id, course_id, title, type, is_trial, content, COALESCE(media_version_id, ''), sort_order
		FROM lessons
		WHERE course_id = $1
		ORDER BY sort_order ASC, created_at ASC, id ASC
	`, courseID)
	if err != nil {
		return nil, 0, err
	}
	defer lsnRows.Close()

	lessonCount := 0
	for lsnRows.Next() {
		var l coursesdomain.Lesson
		if err := lsnRows.Scan(&l.ID, &l.SectionID, &l.CourseID, &l.Title, &l.Type, &l.IsTrial, &l.Content, &l.MediaVersionID, &l.SortOrder); err != nil {
			return nil, 0, err
		}
		if idx, ok := secIndex[l.SectionID]; ok {
			sections[idx].Lessons = append(sections[idx].Lessons, l)
		}
		lessonCount++
	}
	if err := lsnRows.Err(); err != nil {
		return nil, 0, err
	}

	return sections, lessonCount, nil
}

func scanCourseRow(row pgx.Row) (*coursesdomain.Course, error) {
	var c coursesdomain.Course
	err := row.Scan(
		&c.ID, &c.OwnerID, &c.AuthorNickname, &c.Title, &c.Description, &c.CoverURL,
		&c.Category, &c.IsFree, &c.ProductionState, &c.SaleState, &c.DeliveryState,
		&c.ReviewReason, &c.PublishedVersion, &c.Version, &c.FirstDeliverableAt,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, coursesdomain.ErrCourseNotFound
		}
		return nil, err
	}
	c.Sections = make([]coursesdomain.Section, 0)
	return &c, nil
}

const courseSelectCols = `
	id, owner_id, author_nickname, title, description, cover_url,
	category, is_free, production_state, sale_state, delivery_state,
	review_reason, published_version, version, first_deliverable_at,
	created_at, updated_at
`

func (s *PostgresStore) CreateCourse(ctx context.Context, ownerID, authorNickname, title, description, coverURL, category string, isFree bool) (*coursesdomain.Course, error) {
	cat, err := coursesdomain.NormalizeCategory(category)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	id := coursesapp.NewID("crs")

	row := s.pool.QueryRow(ctx, `
		INSERT INTO courses (
			id, owner_id, author_nickname, title, description, cover_url, category,
			is_free, production_state, sale_state, delivery_state, review_reason,
			published_version, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, 'draft', 'for_sale', 'available', '',
			0, 1, $9, $9
		)
		RETURNING `+courseSelectCols,
		id, ownerID, authorNickname, title, description, coverURL, cat, isFree, now,
	)
	return scanCourseRow(row)
}

func (s *PostgresStore) GetCourse(ctx context.Context, courseID string) (*coursesdomain.Course, error) {
	c, err := scanCourseRow(s.pool.QueryRow(ctx, `SELECT `+courseSelectCols+` FROM courses WHERE id = $1`, courseID))
	if err != nil {
		return nil, err
	}
	sections, count, err := s.loadSectionsAndLessons(ctx, courseID)
	if err != nil {
		return nil, err
	}
	c.Sections = sections
	c.LessonCount = count
	return c, nil
}

func (s *PostgresStore) GetLesson(ctx context.Context, lessonID string) (*coursesdomain.Lesson, *coursesdomain.Course, error) {
	var l coursesdomain.Lesson
	err := s.pool.QueryRow(ctx, `
		SELECT id, section_id, course_id, title, type, is_trial, content, COALESCE(media_version_id, ''), sort_order
		FROM lessons
		WHERE id = $1
	`, lessonID).Scan(&l.ID, &l.SectionID, &l.CourseID, &l.Title, &l.Type, &l.IsTrial, &l.Content, &l.MediaVersionID, &l.SortOrder)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, coursesdomain.ErrLessonNotFound
		}
		return nil, nil, err
	}
	course, err := s.GetCourse(ctx, l.CourseID)
	if err != nil {
		return nil, nil, err
	}
	return &l, course, nil
}

func (s *PostgresStore) ListCourses(ctx context.Context, filter coursesdomain.ListFilter) ([]*coursesdomain.Course, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if filter.PublishedOnly {
		conditions = append(conditions, "production_state = 'approved' AND published_version > 0 AND delivery_state <> 'withdrawn'")
	}
	if filter.OwnerID != "" {
		conditions = append(conditions, fmt.Sprintf("owner_id = $%d", argIdx))
		args = append(args, filter.OwnerID)
		argIdx++
	}
	if cat := strings.ToLower(strings.TrimSpace(filter.Category)); cat != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(category) = $%d", argIdx))
		args = append(args, cat)
		argIdx++
	}
	if state := strings.ToLower(strings.TrimSpace(filter.State)); state != "" {
		switch state {
		case "draft", "pending_review", "approved", "rejected":
			conditions = append(conditions, fmt.Sprintf("production_state = $%d", argIdx))
			args = append(args, state)
			argIdx++
		case "closed", "sales_closed":
			conditions = append(conditions, "sale_state = 'closed'")
		case "withdrawn", "unavailable":
			conditions = append(conditions, fmt.Sprintf("delivery_state = $%d", argIdx))
			args = append(args, state)
			argIdx++
		default:
			conditions = append(conditions, fmt.Sprintf("(production_state = $%d OR sale_state = $%d OR delivery_state = $%d)", argIdx, argIdx, argIdx))
			args = append(args, state)
			argIdx++
		}
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+q+"%")
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM courses"+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	query := fmt.Sprintf("SELECT %s FROM courses%s ORDER BY updated_at DESC, id DESC LIMIT $%d OFFSET $%d",
		courseSelectCols, whereClause, argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*coursesdomain.Course, 0)
	for rows.Next() {
		var c coursesdomain.Course
		if err := rows.Scan(
			&c.ID, &c.OwnerID, &c.AuthorNickname, &c.Title, &c.Description, &c.CoverURL,
			&c.Category, &c.IsFree, &c.ProductionState, &c.SaleState, &c.DeliveryState,
			&c.ReviewReason, &c.PublishedVersion, &c.Version, &c.FirstDeliverableAt,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	for _, c := range list {
		sections, count, err := s.loadSectionsAndLessons(ctx, c.ID)
		if err != nil {
			return nil, 0, err
		}
		c.Sections = sections
		c.LessonCount = count
	}

	return list, total, nil
}

func (s *PostgresStore) UpdateCourse(ctx context.Context, courseID, actorID string, isAdmin bool, title, description, coverURL, category string, isFree *bool, expectedVersion int) (*coursesdomain.Course, error) {
	var normalizedCat string
	if category != "" {
		var err error
		normalizedCat, err = coursesdomain.NormalizeCategory(category)
		if err != nil {
			return nil, err
		}
	}

	existing, err := s.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if !coursesapp.CanMutateCourse(existing, actorID, isAdmin) {
		return nil, coursesdomain.ErrPermissionDenied
	}
	if existing.Version != expectedVersion {
		return nil, coursesdomain.ErrVersionConflict
	}

	newTitle := existing.Title
	if title != "" {
		newTitle = title
	}
	newDesc := existing.Description
	if description != "" {
		newDesc = description
	}
	newCover := existing.CoverURL
	if coverURL != "" {
		newCover = coverURL
	}
	newCat := existing.Category
	if normalizedCat != "" {
		newCat = normalizedCat
	}
	newIsFree := existing.IsFree
	if isFree != nil {
		newIsFree = *isFree
	}

	now := time.Now().UTC()
	row := s.pool.QueryRow(ctx, `
		UPDATE courses
		SET title = $1, description = $2, cover_url = $3, category = $4, is_free = $5,
		    version = version + 1, updated_at = $6
		WHERE id = $7 AND version = $8
		RETURNING `+courseSelectCols,
		newTitle, newDesc, newCover, newCat, newIsFree, now, courseID, expectedVersion,
	)
	updated, err := scanCourseRow(row)
	if err != nil {
		if errors.Is(err, coursesdomain.ErrCourseNotFound) {
			return nil, coursesdomain.ErrVersionConflict
		}
		return nil, err
	}
	updated.Sections = existing.Sections
	updated.LessonCount = existing.LessonCount
	return updated, nil
}

func (s *PostgresStore) DeleteCourse(ctx context.Context, courseID, actorID string, isAdmin bool, expectedVersion int) error {
	existing, err := s.GetCourse(ctx, courseID)
	if err != nil {
		return err
	}
	if !coursesapp.CanMutateCourse(existing, actorID, isAdmin) {
		return coursesdomain.ErrPermissionDenied
	}
	if existing.Version != expectedVersion {
		return coursesdomain.ErrVersionConflict
	}
	if existing.PublishedVersion > 0 {
		return coursesdomain.ErrCoursePublished
	}

	tag, err := s.pool.Exec(ctx, `DELETE FROM courses WHERE id = $1 AND version = $2 AND published_version = 0`, courseID, expectedVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return coursesdomain.ErrVersionConflict
	}
	return nil
}

func (s *PostgresStore) SubmitCourse(ctx context.Context, courseID, actorID string, isAdmin bool, expectedVersion int) (*coursesdomain.Course, error) {
	existing, err := s.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if !coursesapp.CanMutateCourse(existing, actorID, isAdmin) {
		return nil, coursesdomain.ErrPermissionDenied
	}
	if existing.Version != expectedVersion {
		return nil, coursesdomain.ErrVersionConflict
	}
	if existing.ProductionState != "draft" && existing.ProductionState != "rejected" {
		return nil, coursesdomain.ErrInvalidState
	}

	now := time.Now().UTC()
	row := s.pool.QueryRow(ctx, `
		UPDATE courses
		SET production_state = 'pending_review', review_reason = '', version = version + 1, updated_at = $1
		WHERE id = $2 AND version = $3 AND production_state IN ('draft', 'rejected')
		RETURNING `+courseSelectCols,
		now, courseID, expectedVersion,
	)
	updated, err := scanCourseRow(row)
	if err != nil {
		if errors.Is(err, coursesdomain.ErrCourseNotFound) {
			return nil, coursesdomain.ErrVersionConflict
		}
		return nil, err
	}
	updated.Sections = existing.Sections
	updated.LessonCount = existing.LessonCount
	return updated, nil
}

func (s *PostgresStore) ReviewCourse(ctx context.Context, courseID string, decision string, reason string, expectedVersion int) (*coursesdomain.Course, error) {
	if decision != "approved" && decision != "rejected" {
		return nil, coursesdomain.ErrInvalidState
	}
	existing, err := s.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if existing.Version != expectedVersion {
		return nil, coursesdomain.ErrVersionConflict
	}
	if existing.ProductionState != "pending_review" {
		return nil, coursesdomain.ErrInvalidState
	}

	now := time.Now().UTC()
	var row pgx.Row
	if decision == "approved" {
		row = s.pool.QueryRow(ctx, `
			UPDATE courses
			SET production_state = 'approved',
			    review_reason = $1,
			    published_version = version,
			    first_deliverable_at = COALESCE(first_deliverable_at, $2),
			    version = version + 1,
			    updated_at = $2
			WHERE id = $3 AND version = $4 AND production_state = 'pending_review'
			RETURNING `+courseSelectCols,
			reason, now, courseID, expectedVersion,
		)
	} else {
		row = s.pool.QueryRow(ctx, `
			UPDATE courses
			SET production_state = 'rejected',
			    review_reason = $1,
			    version = version + 1,
			    updated_at = $2
			WHERE id = $3 AND version = $4 AND production_state = 'pending_review'
			RETURNING `+courseSelectCols,
			reason, now, courseID, expectedVersion,
		)
	}
	updated, err := scanCourseRow(row)
	if err != nil {
		if errors.Is(err, coursesdomain.ErrCourseNotFound) {
			return nil, coursesdomain.ErrVersionConflict
		}
		return nil, err
	}
	updated.Sections = existing.Sections
	updated.LessonCount = existing.LessonCount
	return updated, nil
}

func (s *PostgresStore) CloseSales(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*coursesdomain.Course, error) {
	existing, err := s.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if existing.Version != expectedVersion {
		return nil, coursesdomain.ErrVersionConflict
	}

	now := time.Now().UTC()
	row := s.pool.QueryRow(ctx, `
		UPDATE courses
		SET sale_state = 'closed',
		    review_reason = CASE WHEN $1 <> '' THEN $1 ELSE review_reason END,
		    version = version + 1,
		    updated_at = $2
		WHERE id = $3 AND version = $4
		RETURNING `+courseSelectCols,
		reason, now, courseID, expectedVersion,
	)
	updated, err := scanCourseRow(row)
	if err != nil {
		if errors.Is(err, coursesdomain.ErrCourseNotFound) {
			return nil, coursesdomain.ErrVersionConflict
		}
		return nil, err
	}
	updated.Sections = existing.Sections
	updated.LessonCount = existing.LessonCount
	return updated, nil
}

func (s *PostgresStore) WithdrawCourse(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*coursesdomain.Course, error) {
	existing, err := s.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if existing.Version != expectedVersion {
		return nil, coursesdomain.ErrVersionConflict
	}

	now := time.Now().UTC()
	row := s.pool.QueryRow(ctx, `
		UPDATE courses
		SET delivery_state = 'withdrawn',
		    sale_state = 'closed',
		    review_reason = $1,
		    version = version + 1,
		    updated_at = $2
		WHERE id = $3 AND version = $4
		RETURNING `+courseSelectCols,
		reason, now, courseID, expectedVersion,
	)
	updated, err := scanCourseRow(row)
	if err != nil {
		if errors.Is(err, coursesdomain.ErrCourseNotFound) {
			return nil, coursesdomain.ErrVersionConflict
		}
		return nil, err
	}
	updated.Sections = existing.Sections
	updated.LessonCount = existing.LessonCount
	return updated, nil
}

func (s *PostgresStore) AddSection(ctx context.Context, courseID, title string, sortOrder int) (*coursesdomain.Section, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	tag, err := tx.Exec(ctx, `UPDATE courses SET version = version + 1, updated_at = $1 WHERE id = $2`, now, courseID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, coursesdomain.ErrCourseNotFound
	}

	sec := coursesdomain.Section{
		ID:        coursesapp.NewID("sec"),
		CourseID:  courseID,
		Title:     title,
		SortOrder: sortOrder,
		Lessons:   make([]coursesdomain.Lesson, 0),
	}
	if _, err := tx.Exec(ctx, `INSERT INTO sections (id, course_id, title, sort_order, created_at) VALUES ($1, $2, $3, $4, $5)`,
		sec.ID, sec.CourseID, sec.Title, sec.SortOrder, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &sec, nil
}

func (s *PostgresStore) UpdateSection(ctx context.Context, courseID, sectionID string, title *string, sortOrder *int) (*coursesdomain.Section, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var sec coursesdomain.Section
	err = tx.QueryRow(ctx, `SELECT id, course_id, title, sort_order FROM sections WHERE id = $1 AND course_id = $2`, sectionID, courseID).
		Scan(&sec.ID, &sec.CourseID, &sec.Title, &sec.SortOrder)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1)`, courseID).Scan(&exists)
			if !exists {
				return nil, coursesdomain.ErrCourseNotFound
			}
			return nil, coursesdomain.ErrSectionNotFound
		}
		return nil, err
	}

	if title != nil && strings.TrimSpace(*title) != "" {
		sec.Title = strings.TrimSpace(*title)
	}
	if sortOrder != nil {
		sec.SortOrder = *sortOrder
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE sections SET title = $1, sort_order = $2 WHERE id = $3`, sec.Title, sec.SortOrder, sec.ID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE courses SET version = version + 1, updated_at = $1 WHERE id = $2`, now, courseID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	sec.Lessons = make([]coursesdomain.Lesson, 0)
	return &sec, nil
}

func (s *PostgresStore) DeleteSection(ctx context.Context, courseID, sectionID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `DELETE FROM sections WHERE id = $1 AND course_id = $2`, sectionID, courseID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1)`, courseID).Scan(&exists)
		if !exists {
			return coursesdomain.ErrCourseNotFound
		}
		return coursesdomain.ErrSectionNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE courses SET version = version + 1, updated_at = $1 WHERE id = $2`, time.Now().UTC(), courseID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) AddLesson(ctx context.Context, courseID, sectionID, title, lessonType string, isTrial bool, content, mediaVersionID string, sortOrder int) (*coursesdomain.Lesson, error) {
	validType, err := coursesdomain.ValidateLessonType(lessonType)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var secExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sections WHERE id = $1 AND course_id = $2)`, sectionID, courseID).Scan(&secExists); err != nil {
		return nil, err
	}
	if !secExists {
		var crsExists bool
		_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1)`, courseID).Scan(&crsExists)
		if !crsExists {
			return nil, coursesdomain.ErrCourseNotFound
		}
		return nil, coursesdomain.ErrSectionNotFound
	}

	now := time.Now().UTC()
	l := coursesdomain.Lesson{
		ID:             coursesapp.NewID("lsn"),
		SectionID:      sectionID,
		CourseID:       courseID,
		Title:          title,
		Type:           validType,
		IsTrial:        isTrial,
		Content:        content,
		MediaVersionID: mediaVersionID,
		SortOrder:      sortOrder,
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO lessons (id, section_id, course_id, title, type, is_trial, content, media_version_id, sort_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, l.ID, l.SectionID, l.CourseID, l.Title, l.Type, l.IsTrial, l.Content, l.MediaVersionID, l.SortOrder, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE courses SET version = version + 1, updated_at = $1 WHERE id = $2`, now, courseID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *PostgresStore) UpdateLesson(ctx context.Context, courseID, lessonID string, patch coursesdomain.LessonPatch) (*coursesdomain.Lesson, error) {
	if patch.Type != nil {
		validType, err := coursesdomain.ValidateLessonType(*patch.Type)
		if err != nil {
			return nil, err
		}
		patch.Type = &validType
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var l coursesdomain.Lesson
	err = tx.QueryRow(ctx, `
		SELECT id, section_id, course_id, title, type, is_trial, content, COALESCE(media_version_id, ''), sort_order
		FROM lessons
		WHERE id = $1 AND course_id = $2
	`, lessonID, courseID).Scan(&l.ID, &l.SectionID, &l.CourseID, &l.Title, &l.Type, &l.IsTrial, &l.Content, &l.MediaVersionID, &l.SortOrder)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var crsExists bool
			_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1)`, courseID).Scan(&crsExists)
			if !crsExists {
				return nil, coursesdomain.ErrCourseNotFound
			}
			return nil, coursesdomain.ErrLessonNotFound
		}
		return nil, err
	}

	if patch.SectionID != nil && *patch.SectionID != "" && *patch.SectionID != l.SectionID {
		var secExists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sections WHERE id = $1 AND course_id = $2)`, *patch.SectionID, courseID).Scan(&secExists); err != nil {
			return nil, err
		}
		if !secExists {
			return nil, coursesdomain.ErrSectionNotFound
		}
		l.SectionID = *patch.SectionID
	}
	if patch.Title != nil && strings.TrimSpace(*patch.Title) != "" {
		l.Title = strings.TrimSpace(*patch.Title)
	}
	if patch.Type != nil {
		l.Type = *patch.Type
	}
	if patch.IsTrial != nil {
		l.IsTrial = *patch.IsTrial
	}
	if patch.Content != nil {
		l.Content = *patch.Content
	}
	if patch.MediaVersionID != nil {
		l.MediaVersionID = *patch.MediaVersionID
	}
	if patch.SortOrder != nil {
		l.SortOrder = *patch.SortOrder
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE lessons
		SET section_id = $1, title = $2, type = $3, is_trial = $4, content = $5, media_version_id = $6, sort_order = $7
		WHERE id = $8 AND course_id = $9
	`, l.SectionID, l.Title, l.Type, l.IsTrial, l.Content, l.MediaVersionID, l.SortOrder, l.ID, courseID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE courses SET version = version + 1, updated_at = $1 WHERE id = $2`, now, courseID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *PostgresStore) DeleteLesson(ctx context.Context, courseID, lessonID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `DELETE FROM lessons WHERE id = $1 AND course_id = $2`, lessonID, courseID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var crsExists bool
		_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1)`, courseID).Scan(&crsExists)
		if !crsExists {
			return coursesdomain.ErrCourseNotFound
		}
		return coursesdomain.ErrLessonNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE courses SET version = version + 1, updated_at = $1 WHERE id = $2`, time.Now().UTC(), courseID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) SetLessonMediaVersion(ctx context.Context, lessonID, mediaVersionID string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE lessons SET media_version_id = $1 WHERE id = $2`, mediaVersionID, lessonID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return coursesdomain.ErrLessonNotFound
	}
	return nil
}

func (s *PostgresStore) EnrollUser(ctx context.Context, userID, courseID, enrollType string) (*coursesdomain.Enrollment, error) {
	c, err := s.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if c.DeliveryState == "withdrawn" {
		return nil, coursesdomain.ErrCourseWithdrawn
	}
	if c.ProductionState != "approved" || c.PublishedVersion == 0 {
		return nil, coursesdomain.ErrCourseUnpublished
	}
	if enrollType == "free" && !c.IsFree {
		return nil, coursesdomain.ErrCourseNotFree
	}

	enr := coursesdomain.Enrollment{
		ID:        coursesapp.NewID("enr"),
		UserID:    userID,
		CourseID:  courseID,
		Type:      enrollType,
		CreatedAt: time.Now().UTC(),
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO enrollments (id, user_id, course_id, type, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, enr.ID, enr.UserID, enr.CourseID, enr.Type, enr.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, coursesdomain.ErrAlreadyEnrolled
		}
		return nil, err
	}
	return &enr, nil
}

func (s *PostgresStore) IsUserEnrolled(ctx context.Context, userID, courseID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM enrollments WHERE user_id = $1 AND course_id = $2)`, userID, courseID).Scan(&exists)
	return exists, err
}

func (s *PostgresStore) ListUserEnrollments(ctx context.Context, userID string) ([]coursesdomain.Enrollment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, course_id, type, created_at
		FROM enrollments
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]coursesdomain.Enrollment, 0)
	for rows.Next() {
		var e coursesdomain.Enrollment
		if err := rows.Scan(&e.ID, &e.UserID, &e.CourseID, &e.Type, &e.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

func (s *PostgresStore) ListUserCourses(ctx context.Context, userID string) ([]*coursesdomain.Course, error) {
	enrollments, err := s.ListUserEnrollments(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]*coursesdomain.Course, 0, len(enrollments))
	for _, e := range enrollments {
		c, err := s.GetCourse(ctx, e.CourseID)
		if err != nil {
			if errors.Is(err, coursesdomain.ErrCourseNotFound) {
				continue
			}
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}

func (s *PostgresStore) GetDeliveryEvidence(ctx context.Context, courseID string, notBefore *time.Time) (firstDeliverableAt *time.Time, evidenceVersion int, contentVersion int, err error) {
	c, err := s.GetCourse(ctx, courseID)
	if err != nil {
		return nil, 0, 0, err
	}
	var firstAt *time.Time
	if c.FirstDeliverableAt != nil {
		t := *c.FirstDeliverableAt
		if notBefore != nil && t.Before(*notBefore) {
			nb := *notBefore
			firstAt = &nb
		} else {
			firstAt = &t
		}
	}
	return firstAt, 1, c.PublishedVersion, nil
}
