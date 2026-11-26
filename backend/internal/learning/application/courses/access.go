package courses

import (
	"context"
	"strings"

	coursesdomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/courses"
)

// CheckUserCourseAccess evaluates whether a user has current content access to a course.
// Returns (allowed, reason, error). A non-nil error indicates a downstream dependency failure (fail-closed).
func CheckUserCourseAccess(ctx context.Context, store Store, entitlements EntitlementClient, userID string, roles []string, course *coursesdomain.Course, purpose string) (bool, string, error) {
	if course == nil {
		return false, "course_not_found", nil
	}
	if course.DeliveryState == "withdrawn" {
		return false, "course_withdrawn", nil
	}
	if course.DeliveryState != "available" {
		return false, "course_unavailable", nil
	}
	if userID == "" {
		return false, "unauthenticated", nil
	}
	if course.OwnerID == userID {
		return true, "course_owner", nil
	}
	for _, r := range roles {
		if strings.EqualFold(strings.TrimSpace(r), "admin") {
			return true, "admin", nil
		}
	}
	if course.ProductionState != "approved" || course.PublishedVersion == 0 {
		return false, "course_not_published", nil
	}

	if course.IsFree {
		enrolled, err := store.IsUserEnrolled(ctx, userID, course.ID)
		if err != nil {
			return false, "enrollment_check_failed", err
		}
		if enrolled {
			return true, "free_enrolled", nil
		}
		return false, "not_enrolled", nil
	}

	if entitlements == nil {
		return false, "entitlement_required", nil
	}
	if purpose == "" {
		purpose = "playback"
	}
	allowed, reason, err := entitlements.CheckEntitlement(ctx, userID, course.ID, purpose)
	if err != nil {
		return false, "entitlement_service_unavailable", err
	}
	if allowed {
		if reason == "" {
			reason = "entitlement_active"
		}
		return true, reason, nil
	}
	if reason == "" {
		reason = "entitlement_required"
	}
	return false, reason, nil
}
