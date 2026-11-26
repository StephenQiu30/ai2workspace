package courses

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	coursesdomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/courses"
)

// EntitlementClient queries commerce for paid course entitlements.
type EntitlementClient interface {
	CheckEntitlement(ctx context.Context, userID, courseID, purpose string) (allowed bool, reason string, err error)
	ListActiveCourseIDs(ctx context.Context, userID string) ([]string, error)
}

// CourseProgressSummary summarizes a user's progress within a course for "My Courses".
type CourseProgressSummary struct {
	LastLessonID         string     `json:"lastLessonId,omitempty"`
	LastPositionSeconds  int        `json:"lastPositionSeconds"`
	WatchedLessonCount   int        `json:"watchedLessonCount"`
	CompletedLessonCount int        `json:"completedLessonCount"`
	TotalLessonCount     int        `json:"totalLessonCount"`
	LastStudiedAt        *time.Time `json:"lastStudiedAt,omitempty"`
}

// ProgressSummaryProvider computes a user's learning progress summary for a given course.
type ProgressSummaryProvider func(ctx context.Context, userID string, course *coursesdomain.Course) (CourseProgressSummary, error)

// Store defines persistence for the learning domain.
type Store interface {
	CreateCourse(ctx context.Context, ownerID, authorNickname, title, description, coverURL, category string, isFree bool) (*coursesdomain.Course, error)
	GetCourse(ctx context.Context, courseID string) (*coursesdomain.Course, error)
	GetLesson(ctx context.Context, lessonID string) (*coursesdomain.Lesson, *coursesdomain.Course, error)
	ListCourses(ctx context.Context, filter coursesdomain.ListFilter) ([]*coursesdomain.Course, int, error)
	UpdateCourse(ctx context.Context, courseID, actorID string, isAdmin bool, title, description, coverURL, category string, isFree *bool, expectedVersion int) (*coursesdomain.Course, error)
	DeleteCourse(ctx context.Context, courseID, actorID string, isAdmin bool, expectedVersion int) error
	SubmitCourse(ctx context.Context, courseID, actorID string, isAdmin bool, expectedVersion int) (*coursesdomain.Course, error)
	ReviewCourse(ctx context.Context, courseID string, decision string, reason string, expectedVersion int) (*coursesdomain.Course, error)
	CloseSales(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*coursesdomain.Course, error)
	WithdrawCourse(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*coursesdomain.Course, error)
	AddSection(ctx context.Context, courseID, title string, sortOrder int) (*coursesdomain.Section, error)
	UpdateSection(ctx context.Context, courseID, sectionID string, title *string, sortOrder *int) (*coursesdomain.Section, error)
	DeleteSection(ctx context.Context, courseID, sectionID string) error
	AddLesson(ctx context.Context, courseID, sectionID, title, lessonType string, isTrial bool, content, mediaVersionID string, sortOrder int) (*coursesdomain.Lesson, error)
	UpdateLesson(ctx context.Context, courseID, lessonID string, patch coursesdomain.LessonPatch) (*coursesdomain.Lesson, error)
	DeleteLesson(ctx context.Context, courseID, lessonID string) error
	SetLessonMediaVersion(ctx context.Context, lessonID, mediaVersionID string) error
	EnrollUser(ctx context.Context, userID, courseID, enrollType string) (*coursesdomain.Enrollment, error)
	IsUserEnrolled(ctx context.Context, userID, courseID string) (bool, error)
	ListUserEnrollments(ctx context.Context, userID string) ([]coursesdomain.Enrollment, error)
	ListUserCourses(ctx context.Context, userID string) ([]*coursesdomain.Course, error)
	GetDeliveryEvidence(ctx context.Context, courseID string, notBefore *time.Time) (firstDeliverableAt *time.Time, evidenceVersion int, contentVersion int, err error)
}

// MemoryStore is an in-memory thread-safe implementation of Store.
type MemoryStore struct {
	mu          sync.RWMutex
	courses     map[string]*coursesdomain.Course      // courseID -> Course
	enrollments map[string][]coursesdomain.Enrollment // userID -> []Enrollment
}

// NewMemoryStore initializes a new MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		courses:     make(map[string]*coursesdomain.Course),
		enrollments: make(map[string][]coursesdomain.Enrollment),
	}
}

func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func cloneCourse(c *coursesdomain.Course) *coursesdomain.Course {
	if c == nil {
		return nil
	}
	out := *c
	if c.FirstDeliverableAt != nil {
		t := *c.FirstDeliverableAt
		out.FirstDeliverableAt = &t
	}
	sections := make([]coursesdomain.Section, len(c.Sections))
	lessonCount := 0
	for i, sec := range c.Sections {
		secCopy := sec
		lessons := make([]coursesdomain.Lesson, len(sec.Lessons))
		copy(lessons, sec.Lessons)
		sort.SliceStable(lessons, func(a, b int) bool {
			if lessons[a].SortOrder != lessons[b].SortOrder {
				return lessons[a].SortOrder < lessons[b].SortOrder
			}
			return lessons[a].ID < lessons[b].ID
		})
		secCopy.Lessons = lessons
		sections[i] = secCopy
		lessonCount += len(lessons)
	}
	sort.SliceStable(sections, func(a, b int) bool {
		if sections[a].SortOrder != sections[b].SortOrder {
			return sections[a].SortOrder < sections[b].SortOrder
		}
		return sections[a].ID < sections[b].ID
	})
	out.Sections = sections
	out.LessonCount = lessonCount
	return &out
}

func CanMutateCourse(c *coursesdomain.Course, actorID string, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	return actorID != "" && c.OwnerID == actorID
}

// CreateCourse creates a course in draft status.
func (s *MemoryStore) CreateCourse(ctx context.Context, ownerID, authorNickname, title, description, coverURL, category string, isFree bool) (*coursesdomain.Course, error) {
	cat, err := coursesdomain.NormalizeCategory(category)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	course := &coursesdomain.Course{
		ID:               NewID("crs"),
		OwnerID:          ownerID,
		AuthorNickname:   authorNickname,
		Title:            title,
		Description:      description,
		CoverURL:         coverURL,
		Category:         cat,
		IsFree:           isFree,
		ProductionState:  "draft",
		SaleState:        "for_sale",
		DeliveryState:    "available",
		PublishedVersion: 0,
		Version:          1,
		Sections:         make([]coursesdomain.Section, 0),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	s.courses[course.ID] = course
	return cloneCourse(course), nil
}

// GetCourse retrieves a course by ID.
func (s *MemoryStore) GetCourse(ctx context.Context, courseID string) (*coursesdomain.Course, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, coursesdomain.ErrCourseNotFound
	}
	return cloneCourse(c), nil
}

// GetLesson looks up a lesson by ID along with its owning course.
func (s *MemoryStore) GetLesson(ctx context.Context, lessonID string) (*coursesdomain.Lesson, *coursesdomain.Course, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.courses {
		for _, sec := range c.Sections {
			for _, l := range sec.Lessons {
				if l.ID == lessonID {
					lCopy := l
					return &lCopy, cloneCourse(c), nil
				}
			}
		}
	}
	return nil, nil, coursesdomain.ErrLessonNotFound
}

// ListCourses lists courses matching filter with stable UpdatedAt DESC sorting and pagination.
func (s *MemoryStore) ListCourses(ctx context.Context, filter coursesdomain.ListFilter) ([]*coursesdomain.Course, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(filter.Query))
	cat := strings.ToLower(strings.TrimSpace(filter.Category))
	state := strings.ToLower(strings.TrimSpace(filter.State))

	var matched []*coursesdomain.Course
	for _, c := range s.courses {
		if filter.PublishedOnly {
			if c.ProductionState != "approved" || c.PublishedVersion == 0 || c.DeliveryState == "withdrawn" {
				continue
			}
		}
		if filter.OwnerID != "" && c.OwnerID != filter.OwnerID {
			continue
		}
		if cat != "" && strings.ToLower(c.Category) != cat {
			continue
		}
		if state != "" && !matchesStateFilter(c, state) {
			continue
		}
		if q != "" {
			if !strings.Contains(strings.ToLower(c.Title), q) && !strings.Contains(strings.ToLower(c.Description), q) {
				continue
			}
		}
		matched = append(matched, cloneCourse(c))
	}

	sort.SliceStable(matched, func(i, j int) bool {
		if !matched[i].UpdatedAt.Equal(matched[j].UpdatedAt) {
			return matched[i].UpdatedAt.After(matched[j].UpdatedAt)
		}
		return matched[i].ID > matched[j].ID
	})

	total := len(matched)
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

	start := (page - 1) * pageSize
	if start >= total {
		return []*coursesdomain.Course{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return matched[start:end], total, nil
}

func matchesStateFilter(c *coursesdomain.Course, state string) bool {
	switch state {
	case "draft", "pending_review", "approved", "rejected":
		return c.ProductionState == state
	case "closed", "sales_closed":
		return c.SaleState == "closed"
	case "withdrawn", "unavailable":
		return c.DeliveryState == state
	default:
		return c.ProductionState == state || c.SaleState == state || c.DeliveryState == state
	}
}

// UpdateCourse updates course metadata with ownership and optimistic concurrency checks.
func (s *MemoryStore) UpdateCourse(ctx context.Context, courseID, actorID string, isAdmin bool, title, description, coverURL, category string, isFree *bool, expectedVersion int) (*coursesdomain.Course, error) {
	var normalizedCat string
	if category != "" {
		var err error
		normalizedCat, err = coursesdomain.NormalizeCategory(category)
		if err != nil {
			return nil, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, coursesdomain.ErrCourseNotFound
	}
	if !CanMutateCourse(c, actorID, isAdmin) {
		return nil, coursesdomain.ErrPermissionDenied
	}
	if c.Version != expectedVersion {
		return nil, coursesdomain.ErrVersionConflict
	}

	if title != "" {
		c.Title = title
	}
	if description != "" {
		c.Description = description
	}
	if coverURL != "" {
		c.CoverURL = coverURL
	}
	if normalizedCat != "" {
		c.Category = normalizedCat
	}
	if isFree != nil {
		c.IsFree = *isFree
	}

	c.Version++
	c.UpdatedAt = time.Now().UTC()

	return cloneCourse(c), nil
}

// DeleteCourse deletes an unpublished course with ownership and version checks.
func (s *MemoryStore) DeleteCourse(ctx context.Context, courseID, actorID string, isAdmin bool, expectedVersion int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return coursesdomain.ErrCourseNotFound
	}
	if !CanMutateCourse(c, actorID, isAdmin) {
		return coursesdomain.ErrPermissionDenied
	}
	if c.Version != expectedVersion {
		return coursesdomain.ErrVersionConflict
	}
	if c.PublishedVersion > 0 {
		return coursesdomain.ErrCoursePublished
	}

	delete(s.courses, courseID)
	return nil
}

// SubmitCourse transitions a draft or rejected course to pending_review.
func (s *MemoryStore) SubmitCourse(ctx context.Context, courseID, actorID string, isAdmin bool, expectedVersion int) (*coursesdomain.Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, coursesdomain.ErrCourseNotFound
	}
	if !CanMutateCourse(c, actorID, isAdmin) {
		return nil, coursesdomain.ErrPermissionDenied
	}
	if c.Version != expectedVersion {
		return nil, coursesdomain.ErrVersionConflict
	}
	if c.ProductionState != "draft" && c.ProductionState != "rejected" {
		return nil, coursesdomain.ErrInvalidState
	}

	c.ProductionState = "pending_review"
	c.ReviewReason = ""
	c.Version++
	c.UpdatedAt = time.Now().UTC()

	return cloneCourse(c), nil
}

// ReviewCourse processes an admin review decision (approved or rejected) on a pending_review course.
func (s *MemoryStore) ReviewCourse(ctx context.Context, courseID string, decision string, reason string, expectedVersion int) (*coursesdomain.Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, coursesdomain.ErrCourseNotFound
	}
	if c.Version != expectedVersion {
		return nil, coursesdomain.ErrVersionConflict
	}
	if c.ProductionState != "pending_review" {
		return nil, coursesdomain.ErrInvalidState
	}

	now := time.Now().UTC()
	switch decision {
	case "approved":
		c.ProductionState = "approved"
		c.ReviewReason = reason
		c.PublishedVersion = c.Version
		if c.FirstDeliverableAt == nil {
			c.FirstDeliverableAt = &now
		}
	case "rejected":
		c.ProductionState = "rejected"
		c.ReviewReason = reason
	default:
		return nil, coursesdomain.ErrInvalidState
	}

	c.Version++
	c.UpdatedAt = now

	return cloneCourse(c), nil
}

// CloseSales stops new sales of the course, keeping existing enrolled access intact.
func (s *MemoryStore) CloseSales(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*coursesdomain.Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, coursesdomain.ErrCourseNotFound
	}
	if c.Version != expectedVersion {
		return nil, coursesdomain.ErrVersionConflict
	}

	c.SaleState = "closed"
	if reason != "" {
		c.ReviewReason = reason
	}
	c.Version++
	c.UpdatedAt = time.Now().UTC()

	return cloneCourse(c), nil
}

// WithdrawCourse withdraws a course completely from delivery.
func (s *MemoryStore) WithdrawCourse(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*coursesdomain.Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, coursesdomain.ErrCourseNotFound
	}
	if c.Version != expectedVersion {
		return nil, coursesdomain.ErrVersionConflict
	}

	c.DeliveryState = "withdrawn"
	c.SaleState = "closed"
	c.ReviewReason = reason
	c.Version++
	c.UpdatedAt = time.Now().UTC()

	return cloneCourse(c), nil
}

// AddSection appends a new chapter section to the course.
func (s *MemoryStore) AddSection(ctx context.Context, courseID, title string, sortOrder int) (*coursesdomain.Section, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, coursesdomain.ErrCourseNotFound
	}

	sec := coursesdomain.Section{
		ID:        NewID("sec"),
		CourseID:  courseID,
		Title:     title,
		SortOrder: sortOrder,
		Lessons:   make([]coursesdomain.Lesson, 0),
	}

	c.Sections = append(c.Sections, sec)
	c.Version++
	c.UpdatedAt = time.Now().UTC()

	secCopy := sec
	return &secCopy, nil
}

// UpdateSection modifies a section's title or sort order.
func (s *MemoryStore) UpdateSection(ctx context.Context, courseID, sectionID string, title *string, sortOrder *int) (*coursesdomain.Section, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, coursesdomain.ErrCourseNotFound
	}

	for i := range c.Sections {
		if c.Sections[i].ID == sectionID {
			if title != nil && strings.TrimSpace(*title) != "" {
				c.Sections[i].Title = strings.TrimSpace(*title)
			}
			if sortOrder != nil {
				c.Sections[i].SortOrder = *sortOrder
			}
			c.Version++
			c.UpdatedAt = time.Now().UTC()
			secCopy := c.Sections[i]
			secCopy.Lessons = append([]coursesdomain.Lesson(nil), c.Sections[i].Lessons...)
			return &secCopy, nil
		}
	}

	return nil, coursesdomain.ErrSectionNotFound
}

// DeleteSection removes a section and its lessons from a course.
func (s *MemoryStore) DeleteSection(ctx context.Context, courseID, sectionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return coursesdomain.ErrCourseNotFound
	}

	for i := range c.Sections {
		if c.Sections[i].ID == sectionID {
			c.Sections = append(c.Sections[:i], c.Sections[i+1:]...)
			c.Version++
			c.UpdatedAt = time.Now().UTC()
			return nil
		}
	}

	return coursesdomain.ErrSectionNotFound
}

// AddLesson appends a lesson unit to the section.
func (s *MemoryStore) AddLesson(ctx context.Context, courseID, sectionID, title, lessonType string, isTrial bool, content, mediaVersionID string, sortOrder int) (*coursesdomain.Lesson, error) {
	validType, err := coursesdomain.ValidateLessonType(lessonType)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, coursesdomain.ErrCourseNotFound
	}

	for i := range c.Sections {
		if c.Sections[i].ID == sectionID {
			l := coursesdomain.Lesson{
				ID:             NewID("lsn"),
				SectionID:      sectionID,
				CourseID:       courseID,
				Title:          title,
				Type:           validType,
				IsTrial:        isTrial,
				Content:        content,
				MediaVersionID: mediaVersionID,
				SortOrder:      sortOrder,
			}
			c.Sections[i].Lessons = append(c.Sections[i].Lessons, l)
			c.Version++
			c.UpdatedAt = time.Now().UTC()
			lCopy := l
			return &lCopy, nil
		}
	}

	return nil, coursesdomain.ErrSectionNotFound
}

// UpdateLesson updates an existing lesson within a course.
func (s *MemoryStore) UpdateLesson(ctx context.Context, courseID, lessonID string, patch coursesdomain.LessonPatch) (*coursesdomain.Lesson, error) {
	if patch.Type != nil {
		validType, err := coursesdomain.ValidateLessonType(*patch.Type)
		if err != nil {
			return nil, err
		}
		patch.Type = &validType
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, coursesdomain.ErrCourseNotFound
	}

	srcSecIdx := -1
	srcLsnIdx := -1
	for i := range c.Sections {
		for j := range c.Sections[i].Lessons {
			if c.Sections[i].Lessons[j].ID == lessonID {
				srcSecIdx = i
				srcLsnIdx = j
				break
			}
		}
		if srcSecIdx >= 0 {
			break
		}
	}
	if srcSecIdx < 0 {
		return nil, coursesdomain.ErrLessonNotFound
	}

	targetSecIdx := srcSecIdx
	if patch.SectionID != nil && *patch.SectionID != "" && *patch.SectionID != c.Sections[srcSecIdx].ID {
		targetSecIdx = -1
		for i := range c.Sections {
			if c.Sections[i].ID == *patch.SectionID {
				targetSecIdx = i
				break
			}
		}
		if targetSecIdx < 0 {
			return nil, coursesdomain.ErrSectionNotFound
		}
	}

	lesson := c.Sections[srcSecIdx].Lessons[srcLsnIdx]
	if patch.Title != nil && strings.TrimSpace(*patch.Title) != "" {
		lesson.Title = strings.TrimSpace(*patch.Title)
	}
	if patch.Type != nil {
		lesson.Type = *patch.Type
	}
	if patch.IsTrial != nil {
		lesson.IsTrial = *patch.IsTrial
	}
	if patch.Content != nil {
		lesson.Content = *patch.Content
	}
	if patch.MediaVersionID != nil {
		lesson.MediaVersionID = *patch.MediaVersionID
	}
	if patch.SortOrder != nil {
		lesson.SortOrder = *patch.SortOrder
	}

	if targetSecIdx != srcSecIdx {
		lesson.SectionID = c.Sections[targetSecIdx].ID
		c.Sections[srcSecIdx].Lessons = append(c.Sections[srcSecIdx].Lessons[:srcLsnIdx], c.Sections[srcSecIdx].Lessons[srcLsnIdx+1:]...)
		c.Sections[targetSecIdx].Lessons = append(c.Sections[targetSecIdx].Lessons, lesson)
	} else {
		c.Sections[srcSecIdx].Lessons[srcLsnIdx] = lesson
	}

	c.Version++
	c.UpdatedAt = time.Now().UTC()

	lCopy := lesson
	return &lCopy, nil
}

// DeleteLesson removes a lesson from a course.
func (s *MemoryStore) DeleteLesson(ctx context.Context, courseID, lessonID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return coursesdomain.ErrCourseNotFound
	}

	for i := range c.Sections {
		for j := range c.Sections[i].Lessons {
			if c.Sections[i].Lessons[j].ID == lessonID {
				c.Sections[i].Lessons = append(c.Sections[i].Lessons[:j], c.Sections[i].Lessons[j+1:]...)
				c.Version++
				c.UpdatedAt = time.Now().UTC()
				return nil
			}
		}
	}

	return coursesdomain.ErrLessonNotFound
}

// SetLessonMediaVersion binds a published media version ID to a lesson.
func (s *MemoryStore) SetLessonMediaVersion(ctx context.Context, lessonID, mediaVersionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.courses {
		for i := range c.Sections {
			for j := range c.Sections[i].Lessons {
				if c.Sections[i].Lessons[j].ID == lessonID {
					c.Sections[i].Lessons[j].MediaVersionID = mediaVersionID
					c.UpdatedAt = time.Now().UTC()
					return nil
				}
			}
		}
	}
	return coursesdomain.ErrLessonNotFound
}

// EnrollUser registers user enrollment for a course.
func (s *MemoryStore) EnrollUser(ctx context.Context, userID, courseID, enrollType string) (*coursesdomain.Enrollment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, coursesdomain.ErrCourseNotFound
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

	for _, e := range s.enrollments[userID] {
		if e.CourseID == courseID {
			return nil, coursesdomain.ErrAlreadyEnrolled
		}
	}

	enr := coursesdomain.Enrollment{
		ID:        NewID("enr"),
		UserID:    userID,
		CourseID:  courseID,
		Type:      enrollType,
		CreatedAt: time.Now().UTC(),
	}
	s.enrollments[userID] = append(s.enrollments[userID], enr)

	enrCopy := enr
	return &enrCopy, nil
}

// IsUserEnrolled reports whether the user has an enrollment record for the course.
func (s *MemoryStore) IsUserEnrolled(ctx context.Context, userID, courseID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range s.enrollments[userID] {
		if e.CourseID == courseID {
			return true, nil
		}
	}
	return false, nil
}

// ListUserEnrollments returns all enrollment records for the user.
func (s *MemoryStore) ListUserEnrollments(ctx context.Context, userID string) ([]coursesdomain.Enrollment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	src := s.enrollments[userID]
	out := make([]coursesdomain.Enrollment, len(src))
	copy(out, src)
	return out, nil
}

// ListUserCourses returns courses the user has enrolled in locally.
func (s *MemoryStore) ListUserCourses(ctx context.Context, userID string) ([]*coursesdomain.Course, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*coursesdomain.Course
	for _, e := range s.enrollments[userID] {
		if c, exists := s.courses[e.CourseID]; exists {
			result = append(result, cloneCourse(c))
		}
	}
	return result, nil
}

// GetDeliveryEvidence retrieves delivery availability timestamp and versions for commerce refund assessment.
func (s *MemoryStore) GetDeliveryEvidence(ctx context.Context, courseID string, notBefore *time.Time) (firstDeliverableAt *time.Time, evidenceVersion int, contentVersion int, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, 0, 0, coursesdomain.ErrCourseNotFound
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
