// Package access contains request DTOs.
package access

// InviteInstructorRequest defines parameters for inviting an instructor.
type InviteInstructorRequest struct {
	Email string `json:"email" binding:"required,email" example:"instructor@example.com"`
}

// AcceptInvitationRequest defines the parameter to accept an instructor invitation.
type AcceptInvitationRequest struct {
	Token string `json:"token" binding:"required" example:"a1b2c3d4e5f6"`
}

// SetAccountStatusRequest defines parameters to change an account's operational status.
type SetAccountStatusRequest struct {
	Status          string `json:"status" binding:"required" example:"suspended"`
	ExpectedVersion int    `json:"expectedVersion" binding:"required" example:"1"`
	Reason          string `json:"reason" binding:"required" example:"Policy violation"`
}

// RevokeInstructorRequest defines parameters to remove the instructor role.
type RevokeInstructorRequest struct {
	ExpectedVersion int    `json:"expectedVersion" binding:"required" example:"1"`
	Reason          string `json:"reason" example:"Instructor qualification withdrawn"`
}

// ResolveSessionRequest defines the payload for internal service identity resolution.
type ResolveSessionRequest struct {
	OpaqueSession string `json:"opaqueSession" binding:"required" example:"sess_token_12345"`
}
