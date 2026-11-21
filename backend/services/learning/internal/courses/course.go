// Package courses manages courses, sections, lessons, publication lifecycle, and delivery evidence.
package courses

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
	ErrCourseNotFound    = errors.New("course not found")
	ErrSectionNotFound   = errors.New("section not found")
	ErrLessonNotFound    = errors.New("lesson not found")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrVersionConflict   = errors.New("version conflict")
	ErrCourseNotDraft    = errors.New("course is not in draft state")
	ErrCoursePublished   = errors.New("cannot delete published course; use withdraw instead")
	ErrCourseWithdrawn   = errors.New("course has been withdrawn")
	ErrCourseUnpublished = errors.New("course is not published")
	ErrInvalidState      = errors.New("invalid state transition")
	ErrAlreadyEnrolled   = errors.New("user already enrolled in course")
	ErrCourseNotFree     = errors.New("cannot join non-free course directly")
	ErrInvalidLessonType = errors.New("lesson type must be video or article")
	ErrInvalidCategory   = errors.New("invalid category")
)

const DefaultCategory = "general"

// NormalizeCategory cleans and defaults a course category string.
func NormalizeCategory(raw string) (string, error) {
	cat := strings.ToLower(strings.TrimSpace(raw))
	if cat == "" {
		return DefaultCategory, nil
	}
	if len(cat) > 64 {
		return "", ErrInvalidCategory
	}
	for _, r := range cat {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return "", ErrInvalidCategory
		}
	}
	return cat, nil
}

// ValidateLessonType verifies that lessonType is either "video" or "article".
func ValidateLessonType(lessonType string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(lessonType))
	if t != "video" && t != "article" {
		return "", ErrInvalidLessonType
	}
	return t, nil
}

// Lesson represents a single study unit (video or article).
type Lesson struct {
	ID             string `json:"id"`
	SectionID      string `json:"sectionId"`
	CourseID       string `json:"courseId"`
	Title          string `json:"title"`
	Type           string `json:"type"` // "video", "article"
	IsTrial        bool   `json:"isTrial"`
	Content        string `json:"content"`
	MediaVersionID string `json:"mediaVersionId,omitempty"`
	SortOrder      int    `json:"sortOrder"`
}

// Section represents a chapter grouping multiple lessons.
type Section struct {
	ID        string   `json:"id"`
	CourseID  string   `json:"courseId"`
	Title     string   `json:"title"`
	SortOrder int      `json:"sortOrder"`
	Lessons   []Lesson `json:"lessons"`
}

// Course represents a complete course curriculum and delivery lifecycle.
type Course struct {
	ID                 string     `json:"id"`
	OwnerID            string     `json:"ownerId"`
	AuthorNickname     string     `json:"authorNickname"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	CoverURL           string     `json:"coverUrl"`
	Category           string     `json:"category"`
	IsFree             bool       `json:"isFree"`
	ProductionState    string     `json:"productionState"` // "draft", "pending_review", "approved", "rejected"
	SaleState          string     `json:"saleState"`       // "for_sale", "closed"
	DeliveryState      string     `json:"deliveryState"`   // "available", "unavailable", "withdrawn"
	ReviewReason       string     `json:"reviewReason,omitempty"`
	LessonCount        int        `json:"lessonCount"`
	PublishedVersion   int        `json:"publishedVersion"`
	Version            int        `json:"version"`
	FirstDeliverableAt *time.Time `json:"firstDeliverableAt,omitempty"`
	Sections           []Section  `json:"sections"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// Enrollment represents user enrollment in a course.
type Enrollment struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	CourseID  string    `json:"courseId"`
	Type      string    `json:"type"` // "free", "purchased"
	CreatedAt time.Time `json:"createdAt"`
}

// ListFilter specifies filtering and pagination parameters for listing courses.
type ListFilter struct {
	PublishedOnly bool
	OwnerID       string
	Query         string
	Category      string
	State         string
	Page          int
	PageSize      int
}

// LessonPatch specifies partial updates for an existing lesson.
type LessonPatch struct {
	SectionID      *string
	Title          *string
	Type           *string
	IsTrial        *bool
	Content        *string
	MediaVersionID *string
	SortOrder      *int
}

// UserEntitlement represents a purchased course entitlement returned by commerce.
type UserEntitlement struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	CourseID    string    `json:"courseId"`
	OrderID     string    `json:"orderId"`
	Status      string    `json:"status"`
	ActivatedAt time.Time `json:"activatedAt"`
}

// EntitlementClient queries commerce for paid course entitlements.
type EntitlementClient interface {
	CheckEntitlement(ctx context.Context, userID, courseID, purpose string) (allowed bool, reason string, err error)
	ListUserEntitlements(ctx context.Context, userID string) ([]UserEntitlement, error)
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
type ProgressSummaryProvider func(ctx context.Context, userID string, course *Course) (CourseProgressSummary, error)

// Store defines persistence for the learning domain.
type Store interface {
	CreateCourse(ctx context.Context, ownerID, authorNickname, title, description, coverURL, category string, isFree bool) (*Course, error)
	GetCourse(ctx context.Context, courseID string) (*Course, error)
	GetLesson(ctx context.Context, lessonID string) (*Lesson, *Course, error)
	ListCourses(ctx context.Context, filter ListFilter) ([]*Course, int, error)
	UpdateCourse(ctx context.Context, courseID, actorID string, isAdmin bool, title, description, coverURL, category string, isFree *bool, expectedVersion int) (*Course, error)
	DeleteCourse(ctx context.Context, courseID, actorID string, isAdmin bool, expectedVersion int) error
	SubmitCourse(ctx context.Context, courseID, actorID string, isAdmin bool, expectedVersion int) (*Course, error)
	ReviewCourse(ctx context.Context, courseID string, decision string, reason string, expectedVersion int) (*Course, error)
	CloseSales(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*Course, error)
	WithdrawCourse(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*Course, error)
	AddSection(ctx context.Context, courseID, title string, sortOrder int) (*Section, error)
	UpdateSection(ctx context.Context, courseID, sectionID string, title *string, sortOrder *int) (*Section, error)
	DeleteSection(ctx context.Context, courseID, sectionID string) error
	AddLesson(ctx context.Context, courseID, sectionID, title, lessonType string, isTrial bool, content, mediaVersionID string, sortOrder int) (*Lesson, error)
	UpdateLesson(ctx context.Context, courseID, lessonID string, patch LessonPatch) (*Lesson, error)
	DeleteLesson(ctx context.Context, courseID, lessonID string) error
	SetLessonMediaVersion(ctx context.Context, lessonID, mediaVersionID string) error
	EnrollUser(ctx context.Context, userID, courseID, enrollType string) (*Enrollment, error)
	IsUserEnrolled(ctx context.Context, userID, courseID string) (bool, error)
	ListUserEnrollments(ctx context.Context, userID string) ([]Enrollment, error)
	ListUserCourses(ctx context.Context, userID string) ([]*Course, error)
	GetDeliveryEvidence(ctx context.Context, courseID string, notBefore *time.Time) (firstDeliverableAt *time.Time, evidenceVersion int, contentVersion int, err error)
}

// MemoryStore is an in-memory thread-safe implementation of Store.
type MemoryStore struct {
	mu          sync.RWMutex
	courses     map[string]*Course      // courseID -> Course
	enrollments map[string][]Enrollment // userID -> []Enrollment
}

// NewMemoryStore initializes a new MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		courses:     make(map[string]*Course),
		enrollments: make(map[string][]Enrollment),
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func cloneCourse(c *Course) *Course {
	if c == nil {
		return nil
	}
	out := *c
	if c.FirstDeliverableAt != nil {
		t := *c.FirstDeliverableAt
		out.FirstDeliverableAt = &t
	}
	sections := make([]Section, len(c.Sections))
	lessonCount := 0
	for i, sec := range c.Sections {
		secCopy := sec
		lessons := make([]Lesson, len(sec.Lessons))
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

func canMutateCourse(c *Course, actorID string, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	return actorID != "" && c.OwnerID == actorID
}

// CreateCourse creates a course in draft status.
func (s *MemoryStore) CreateCourse(ctx context.Context, ownerID, authorNickname, title, description, coverURL, category string, isFree bool) (*Course, error) {
	cat, err := NormalizeCategory(category)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	course := &Course{
		ID:               generateID("crs"),
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
		Sections:         make([]Section, 0),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	s.courses[course.ID] = course
	return cloneCourse(course), nil
}

// GetCourse retrieves a course by ID.
func (s *MemoryStore) GetCourse(ctx context.Context, courseID string) (*Course, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}
	return cloneCourse(c), nil
}

// GetLesson looks up a lesson by ID along with its owning course.
func (s *MemoryStore) GetLesson(ctx context.Context, lessonID string) (*Lesson, *Course, error) {
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
	return nil, nil, ErrLessonNotFound
}

// ListCourses lists courses matching filter with stable UpdatedAt DESC sorting and pagination.
func (s *MemoryStore) ListCourses(ctx context.Context, filter ListFilter) ([]*Course, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(filter.Query))
	cat := strings.ToLower(strings.TrimSpace(filter.Category))
	state := strings.ToLower(strings.TrimSpace(filter.State))

	var matched []*Course
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
		return []*Course{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return matched[start:end], total, nil
}

func matchesStateFilter(c *Course, state string) bool {
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
func (s *MemoryStore) UpdateCourse(ctx context.Context, courseID, actorID string, isAdmin bool, title, description, coverURL, category string, isFree *bool, expectedVersion int) (*Course, error) {
	var normalizedCat string
	if category != "" {
		var err error
		normalizedCat, err = NormalizeCategory(category)
		if err != nil {
			return nil, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}
	if !canMutateCourse(c, actorID, isAdmin) {
		return nil, ErrPermissionDenied
	}
	if c.Version != expectedVersion {
		return nil, ErrVersionConflict
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
		return ErrCourseNotFound
	}
	if !canMutateCourse(c, actorID, isAdmin) {
		return ErrPermissionDenied
	}
	if c.Version != expectedVersion {
		return ErrVersionConflict
	}
	if c.PublishedVersion > 0 {
		return ErrCoursePublished
	}

	delete(s.courses, courseID)
	return nil
}

// SubmitCourse transitions a draft or rejected course to pending_review.
func (s *MemoryStore) SubmitCourse(ctx context.Context, courseID, actorID string, isAdmin bool, expectedVersion int) (*Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}
	if !canMutateCourse(c, actorID, isAdmin) {
		return nil, ErrPermissionDenied
	}
	if c.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	if c.ProductionState != "draft" && c.ProductionState != "rejected" {
		return nil, ErrInvalidState
	}

	c.ProductionState = "pending_review"
	c.ReviewReason = ""
	c.Version++
	c.UpdatedAt = time.Now().UTC()

	return cloneCourse(c), nil
}

// ReviewCourse processes an admin review decision (approved or rejected) on a pending_review course.
func (s *MemoryStore) ReviewCourse(ctx context.Context, courseID string, decision string, reason string, expectedVersion int) (*Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}
	if c.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	if c.ProductionState != "pending_review" {
		return nil, ErrInvalidState
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
		return nil, ErrInvalidState
	}

	c.Version++
	c.UpdatedAt = now

	return cloneCourse(c), nil
}

// CloseSales stops new sales of the course, keeping existing enrolled access intact.
func (s *MemoryStore) CloseSales(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}
	if c.Version != expectedVersion {
		return nil, ErrVersionConflict
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
func (s *MemoryStore) WithdrawCourse(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}
	if c.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	c.DeliveryState = "withdrawn"
	c.SaleState = "closed"
	c.ReviewReason = reason
	c.Version++
	c.UpdatedAt = time.Now().UTC()

	return cloneCourse(c), nil
}

// AddSection appends a new chapter section to the course.
func (s *MemoryStore) AddSection(ctx context.Context, courseID, title string, sortOrder int) (*Section, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}

	sec := Section{
		ID:        generateID("sec"),
		CourseID:  courseID,
		Title:     title,
		SortOrder: sortOrder,
		Lessons:   make([]Lesson, 0),
	}

	c.Sections = append(c.Sections, sec)
	c.Version++
	c.UpdatedAt = time.Now().UTC()

	secCopy := sec
	return &secCopy, nil
}

// UpdateSection modifies a section's title or sort order.
func (s *MemoryStore) UpdateSection(ctx context.Context, courseID, sectionID string, title *string, sortOrder *int) (*Section, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
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
			secCopy.Lessons = append([]Lesson(nil), c.Sections[i].Lessons...)
			return &secCopy, nil
		}
	}

	return nil, ErrSectionNotFound
}

// DeleteSection removes a section and its lessons from a course.
func (s *MemoryStore) DeleteSection(ctx context.Context, courseID, sectionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return ErrCourseNotFound
	}

	for i := range c.Sections {
		if c.Sections[i].ID == sectionID {
			c.Sections = append(c.Sections[:i], c.Sections[i+1:]...)
			c.Version++
			c.UpdatedAt = time.Now().UTC()
			return nil
		}
	}

	return ErrSectionNotFound
}

// AddLesson appends a lesson unit to the section.
func (s *MemoryStore) AddLesson(ctx context.Context, courseID, sectionID, title, lessonType string, isTrial bool, content, mediaVersionID string, sortOrder int) (*Lesson, error) {
	validType, err := ValidateLessonType(lessonType)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}

	for i := range c.Sections {
		if c.Sections[i].ID == sectionID {
			l := Lesson{
				ID:             generateID("lsn"),
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

	return nil, ErrSectionNotFound
}

// UpdateLesson updates an existing lesson within a course.
func (s *MemoryStore) UpdateLesson(ctx context.Context, courseID, lessonID string, patch LessonPatch) (*Lesson, error) {
	if patch.Type != nil {
		validType, err := ValidateLessonType(*patch.Type)
		if err != nil {
			return nil, err
		}
		patch.Type = &validType
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
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
		return nil, ErrLessonNotFound
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
			return nil, ErrSectionNotFound
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
		return ErrCourseNotFound
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

	return ErrLessonNotFound
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
	return ErrLessonNotFound
}

// EnrollUser registers user enrollment for a course.
func (s *MemoryStore) EnrollUser(ctx context.Context, userID, courseID, enrollType string) (*Enrollment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}
	if c.DeliveryState == "withdrawn" {
		return nil, ErrCourseWithdrawn
	}
	if c.ProductionState != "approved" || c.PublishedVersion == 0 {
		return nil, ErrCourseUnpublished
	}
	if enrollType == "free" && !c.IsFree {
		return nil, ErrCourseNotFree
	}

	for _, e := range s.enrollments[userID] {
		if e.CourseID == courseID {
			return nil, ErrAlreadyEnrolled
		}
	}

	enr := Enrollment{
		ID:        generateID("enr"),
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
func (s *MemoryStore) ListUserEnrollments(ctx context.Context, userID string) ([]Enrollment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	src := s.enrollments[userID]
	out := make([]Enrollment, len(src))
	copy(out, src)
	return out, nil
}

// ListUserCourses returns courses the user has enrolled in locally.
func (s *MemoryStore) ListUserCourses(ctx context.Context, userID string) ([]*Course, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Course
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
		return nil, 0, 0, ErrCourseNotFound
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
