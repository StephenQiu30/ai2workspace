package access

import (
	"time"

	accessdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/access"
)

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
	Status          string `json:"status" binding:"required,oneof=active suspended" example:"suspended"`
	ExpectedVersion int    `json:"expectedVersion" binding:"required" example:"1"`
	Reason          string `json:"reason" binding:"required" example:"Policy violation"`
}

// RevokeInstructorRequest defines parameters to remove the instructor role.
type RevokeInstructorRequest struct {
	ExpectedVersion int    `json:"expectedVersion" binding:"required" example:"1"`
	Reason          string `json:"reason" example:"Instructor qualification withdrawn"`
}

// AccountSummary represents an account item in the admin accounts table.
type AccountSummary struct {
	ID            string    `json:"id" example:"usr_12345"`
	Email         string    `json:"email" example:"user@example.com"`
	Nickname      string    `json:"nickname" example:"alice"`
	EmailVerified bool      `json:"emailVerified" example:"true"`
	Status        string    `json:"status" example:"active"`
	Roles         []string  `json:"roles" example:"student"`
	Version       int       `json:"version" example:"1"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ListAccountsResponse represents a paginated list of accounts for admin management.
type ListAccountsResponse struct {
	Items    []AccountSummary `json:"items"`
	Total    int              `json:"total" example:"1"`
	Page     int              `json:"page" example:"1"`
	PageSize int              `json:"pageSize" example:"20"`
}

// ListInvitationsResponse represents a paginated list of instructor invitations for admin management.
type ListInvitationsResponse struct {
	Items    []accessdomain.Invitation `json:"items"`
	Total    int                       `json:"total" example:"1"`
	Page     int                       `json:"page" example:"1"`
	PageSize int                       `json:"pageSize" example:"50"`
}

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
