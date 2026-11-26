package entitlements

import (
	"errors"
	"time"
)

var (
	ErrEntitlementNotFound = errors.New("entitlement not found")
)

// GrantStatus defines active or revoked state of an entitlement.
type GrantStatus string

const (
	GrantActive  GrantStatus = "active"
	GrantRevoked GrantStatus = "revoked"
)

// Entitlement represents a purchased learning access grant tied to a specific order.
type Entitlement struct {
	ID           string      `json:"id"`
	UserID       string      `json:"userId"`
	CourseID     string      `json:"courseId"`
	OrderID      string      `json:"orderId"`
	Status       GrantStatus `json:"status"`
	GrantVersion int         `json:"grantVersion"`
	ActivatedAt  time.Time   `json:"activatedAt"`
	RevokedAt    *time.Time  `json:"revokedAt,omitempty"`
}
