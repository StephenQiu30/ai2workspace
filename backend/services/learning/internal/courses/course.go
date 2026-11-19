// Package courses manages courses, sections, lessons, publication lifecycle, and delivery evidence.
package courses

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrCourseNotFound   = errors.New("course not found")
	ErrPermissionDenied = errors.New("permission denied")
	ErrVersionConflict  = errors.New("version conflict")
	ErrCourseNotDraft   = errors.New("course is not in draft state")
	ErrInvalidState     = errors.New("invalid state transition")
	ErrAlreadyEnrolled  = errors.New("user already enrolled in course")
	ErrCourseNotFree    = errors.New("cannot join non-free course directly")
)

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
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	CoverURL           string     `json:"coverUrl"`
	IsFree             bool       `json:"isFree"`
	ProductionState    string     `json:"productionState"` // "draft", "pending_review", "approved", "rejected"
	SaleState          string     `json:"saleState"`       // "for_sale", "closed"
	DeliveryState      string     `json:"deliveryState"`   // "available", "unavailable", "withdrawn"
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

// Store defines persistence for the learning domain.
type Store interface {
	CreateCourse(ctx context.Context, ownerID, title, description, coverURL string, isFree bool) (*Course, error)
	GetCourse(ctx context.Context, courseID string) (*Course, error)
	ListCourses(ctx context.Context, publishedOnly bool, page, limit int) ([]*Course, int, error)
	UpdateCourse(ctx context.Context, courseID, ownerID string, title, description, coverURL string, isFree *bool, expectedVersion int) (*Course, error)
	DeleteCourse(ctx context.Context, courseID, ownerID string, expectedVersion int) error
	SubmitCourse(ctx context.Context, courseID, ownerID string, expectedVersion int) (*Course, error)
	ReviewCourse(ctx context.Context, courseID string, decision string, reason string, expectedVersion int) (*Course, error)
	CloseSales(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*Course, error)
	WithdrawCourse(ctx context.Context, courseID, actorID string, reason string, expectedVersion int) (*Course, error)
	AddSection(ctx context.Context, courseID, title string, sortOrder int) (*Section, error)
	AddLesson(ctx context.Context, courseID, sectionID, title, lessonType string, isTrial bool, content, mediaVersionID string, sortOrder int) (*Lesson, error)
	EnrollUser(ctx context.Context, userID, courseID, enrollType string) (*Enrollment, error)
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

// CreateCourse creates a course in draft status.
func (s *MemoryStore) CreateCourse(ctx context.Context, ownerID, title, description, coverURL string, isFree bool) (*Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	course := &Course{
		ID:               generateID("crs"),
		OwnerID:          ownerID,
		Title:            title,
		Description:      description,
		CoverURL:         coverURL,
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
	cCopy := *course
	return &cCopy, nil
}

// GetCourse retrieves a course by ID.
func (s *MemoryStore) GetCourse(ctx context.Context, courseID string) (*Course, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}
	cCopy := *c
	return &cCopy, nil
}

// ListCourses lists courses with optional published filter and pagination.
func (s *MemoryStore) ListCourses(ctx context.Context, publishedOnly bool, page, limit int) ([]*Course, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matched []*Course
	for _, c := range s.courses {
		if publishedOnly {
			if c.ProductionState != "approved" || c.PublishedVersion == 0 {
				continue
			}
		}
		cCopy := *c
		matched = append(matched, &cCopy)
	}

	total := len(matched)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	start := (page - 1) * limit
	if start >= total {
		return []*Course{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	return matched[start:end], total, nil
}

// UpdateCourse updates course title, description, and settings with optimistic concurrency.
func (s *MemoryStore) UpdateCourse(ctx context.Context, courseID, ownerID string, title, description, coverURL string, isFree *bool, expectedVersion int) (*Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}
	if ownerID != "" && c.OwnerID != ownerID {
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
	if isFree != nil {
		c.IsFree = *isFree
	}

	c.Version++
	c.UpdatedAt = time.Now()

	cCopy := *c
	return &cCopy, nil
}

// DeleteCourse deletes a course in draft status.
func (s *MemoryStore) DeleteCourse(ctx context.Context, courseID, ownerID string, expectedVersion int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return ErrCourseNotFound
	}
	if ownerID != "" && c.OwnerID != ownerID {
		return ErrPermissionDenied
	}
	if c.Version != expectedVersion {
		return ErrVersionConflict
	}
	if c.PublishedVersion > 0 {
		return errors.New("cannot delete published course; use withdraw instead")
	}

	delete(s.courses, courseID)
	return nil
}

// SubmitCourse transitions course from draft to pending_review.
func (s *MemoryStore) SubmitCourse(ctx context.Context, courseID, ownerID string, expectedVersion int) (*Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}
	if ownerID != "" && c.OwnerID != ownerID {
		return nil, ErrPermissionDenied
	}
	if c.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	c.ProductionState = "pending_review"
	c.Version++
	c.UpdatedAt = time.Now()

	cCopy := *c
	return &cCopy, nil
}

// ReviewCourse processes an admin review decision (approved or rejected).
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

	now := time.Now()
	if decision == "approved" {
		c.ProductionState = "approved"
		c.PublishedVersion = c.Version
		if c.FirstDeliverableAt == nil {
			c.FirstDeliverableAt = &now
		}
	} else if decision == "rejected" {
		c.ProductionState = "rejected"
	} else {
		return nil, ErrInvalidState
	}

	c.Version++
	c.UpdatedAt = now

	cCopy := *c
	return &cCopy, nil
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
	c.Version++
	c.UpdatedAt = time.Now()

	cCopy := *c
	return &cCopy, nil
}

// WithdrawCourse administrative action withdrawing course from delivery.
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
	c.Version++
	c.UpdatedAt = time.Now()

	cCopy := *c
	return &cCopy, nil
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
	c.UpdatedAt = time.Now()

	secCopy := sec
	return &secCopy, nil
}

// AddLesson appends a lesson unit to the section.
func (s *MemoryStore) AddLesson(ctx context.Context, courseID, sectionID, title, lessonType string, isTrial bool, content, mediaVersionID string, sortOrder int) (*Lesson, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
	}

	var sectionFound bool
	var l Lesson
	for i := range c.Sections {
		if c.Sections[i].ID == sectionID {
			sectionFound = true
			l = Lesson{
				ID:             generateID("lsn"),
				SectionID:      sectionID,
				CourseID:       courseID,
				Title:          title,
				Type:           lessonType,
				IsTrial:        isTrial,
				Content:        content,
				MediaVersionID: mediaVersionID,
				SortOrder:      sortOrder,
			}
			c.Sections[i].Lessons = append(c.Sections[i].Lessons, l)
			break
		}
	}
	if !sectionFound {
		return nil, errors.New("section not found")
	}

	c.Version++
	c.UpdatedAt = time.Now()

	lCopy := l
	return &lCopy, nil
}

// EnrollUser registers user enrollment for a course.
func (s *MemoryStore) EnrollUser(ctx context.Context, userID, courseID, enrollType string) (*Enrollment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.courses[courseID]
	if !exists {
		return nil, ErrCourseNotFound
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
		CreatedAt: time.Now(),
	}
	s.enrollments[userID] = append(s.enrollments[userID], enr)

	enrCopy := enr
	return &enrCopy, nil
}

// ListUserCourses returns courses the user has enrolled in.
func (s *MemoryStore) ListUserCourses(ctx context.Context, userID string) ([]*Course, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Course
	for _, e := range s.enrollments[userID] {
		if c, exists := s.courses[e.CourseID]; exists {
			cCopy := *c
			result = append(result, &cCopy)
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

	firstAt := c.FirstDeliverableAt
	if firstAt != nil && notBefore != nil && firstAt.Before(*notBefore) {
		firstAt = notBefore
	}

	return firstAt, 1, c.PublishedVersion, nil
}
