package entitlements

import (
	"time"
)

// EntitlementResponse represents a serialized student purchase grant.
type EntitlementResponse struct {
	ID           string     `json:"id"`
	UserID       string     `json:"userId"`
	CourseID     string     `json:"courseId"`
	OrderID      string     `json:"orderId"`
	Status       string     `json:"status"`
	GrantVersion int        `json:"grantVersion"`
	ActivatedAt  time.Time  `json:"activatedAt"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
}
