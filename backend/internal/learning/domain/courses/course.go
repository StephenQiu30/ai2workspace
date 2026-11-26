package courses

import (
	"errors"
	"strings"
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
