// Package access contains response DTOs.
package access

// InviteInstructorResponse represents the outcome of creating an instructor invitation.
type InviteInstructorResponse struct {
	InviteID        string `json:"inviteId" example:"inv_12345"`
	Status          string `json:"status" example:"pending"`
	InvitationToken string `json:"invitationToken,omitempty" example:"a1b2c3d4e5f6"`
}

// AcceptInvitationResponse represents the roles granted after accepting an invitation.
type AcceptInvitationResponse struct {
	Roles []string `json:"roles" example:"student,instructor"`
}

// CancelInvitationResponse represents the result of canceling a pending invitation.
type CancelInvitationResponse struct {
	Status  string `json:"status" example:"cancelled"`
	Version int    `json:"version" example:"2"`
}

// SetAccountStatusResponse reflects the updated status and incremented version.
type SetAccountStatusResponse struct {
	AccountStatus  string `json:"accountStatus" example:"suspended"`
	SessionVersion int    `json:"sessionVersion" example:"2"`
}

// RevokeInstructorResponse reflects the updated roles and version after revoking instructor status.
type RevokeInstructorResponse struct {
	Roles          []string `json:"roles" example:"student"`
	AccountVersion int      `json:"accountVersion" example:"2"`
}

// ResolveSessionResponse returns verified actor identity for internal service consumers.
type ResolveSessionResponse struct {
	UserID         string   `json:"userId" example:"usr_12345"`
	Email          string   `json:"email" example:"user@example.com"`
	EmailVerified  bool     `json:"emailVerified" example:"true"`
	Roles          []string `json:"roles" example:"student,instructor"`
	AccountStatus  string   `json:"accountStatus" example:"active"`
	SessionVersion int      `json:"sessionVersion" example:"1"`
}
