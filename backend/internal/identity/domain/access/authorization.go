package access

import (
	"errors"
	"time"
)

var (
	ErrInvitationNotFound   = errors.New("invitation not found")
	ErrInvitationNotPending = errors.New("invitation is not pending")
	ErrInvitationExpired    = errors.New("invitation expired")
	ErrVersionConflict      = errors.New("version conflict")
)

// Invitation tracks an instructor invitation.
type Invitation struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	InvitedBy string    `json:"invitedBy"`
	Status    string    `json:"status"` // "pending", "accepted", "cancelled", "expired"
	TokenHash string    `json:"-"`
	Version   int       `json:"version"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ListInvitationsFilter defines optional filters and pagination for listing instructor invitations.
type ListInvitationsFilter struct {
	Status   string
	Page     int
	PageSize int
}

// AuditLog records administrative and sensitive role/status changes.
type AuditLog struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actorId"`
	TargetID  string    `json:"targetId"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}
