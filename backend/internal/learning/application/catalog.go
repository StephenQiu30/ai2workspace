// Package application implements learning operations used by other modules.
package application

import (
	"context"
	"errors"
	"time"

	coursesapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/courses"
	coursesdomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/courses"
)

// Catalog supplies course facts and access decisions to other modules.
type Catalog struct {
	courseStore  coursesapp.Store
	entitlements coursesapp.EntitlementClient
}

// NewCatalog connects course storage and purchase entitlements.
func NewCatalog(store coursesapp.Store, entitlements coursesapp.EntitlementClient) *Catalog {
	return &Catalog{courseStore: store, entitlements: entitlements}
}

// SaleSnapshot is the course state needed when deciding whether to accept an order.
type SaleSnapshot struct {
	CourseID         string
	Title            string
	IsFree           bool
	SaleState        string
	PublishedVersion int
	Deliverable      bool
}

// SaleSnapshot loads the current sale and delivery state of a course.
func (c *Catalog) SaleSnapshot(ctx context.Context, courseID string) (*SaleSnapshot, error) {
	course, err := c.courseStore.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	return &SaleSnapshot{
		CourseID:         course.ID,
		Title:            course.Title,
		IsFree:           course.IsFree,
		SaleState:        course.SaleState,
		PublishedVersion: course.PublishedVersion,
		Deliverable:      course.ProductionState == "approved" && course.PublishedVersion > 0 && course.DeliveryState == "available",
	}, nil
}

// DeliveryEvidence reports when a course became deliverable for this order.
func (c *Catalog) DeliveryEvidence(ctx context.Context, courseID string, notBefore time.Time) (*time.Time, error) {
	at, _, _, err := c.courseStore.GetDeliveryEvidence(ctx, courseID, &notBefore)
	return at, err
}

// DiscussionAccess reports whether the user can access the course discussion.
func (c *Catalog) DiscussionAccess(ctx context.Context, courseID, userID string) (bool, error) {
	return c.checkAccess(ctx, userID, courseID, "discussion")
}

// KnowledgeAccess reports whether the user can access course knowledge.
func (c *Catalog) KnowledgeAccess(ctx context.Context, userID, courseID string) (bool, error) {
	return c.checkAccess(ctx, userID, courseID, "knowledge")
}

func (c *Catalog) checkAccess(ctx context.Context, userID, courseID, purpose string) (bool, error) {
	course, err := c.courseStore.GetCourse(ctx, courseID)
	if errors.Is(err, coursesdomain.ErrCourseNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	allowed, _, err := coursesapp.CheckUserCourseAccess(ctx, c.courseStore, c.entitlements, userID, nil, course, purpose)
	return allowed, err
}
