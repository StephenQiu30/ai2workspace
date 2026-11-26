package progress

import (
	"errors"
	"time"
)

var (
	ErrProgressNotFound = errors.New("progress not found")
	ErrVersionConflict  = errors.New("optimistic version conflict: stale progress report")
	ErrInvalidPosition  = errors.New("position cannot be negative")
)

// Progress records a user's latest learning position in a lesson.
type Progress struct {
	ID              string    `json:"id"`
	UserID          string    `json:"userId"`
	CourseID        string    `json:"courseId"`
	LessonID        string    `json:"lessonId"`
	ContentVersion  int       `json:"contentVersion"`
	PositionSeconds int       `json:"positionSeconds"`
	Version         int       `json:"version"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
