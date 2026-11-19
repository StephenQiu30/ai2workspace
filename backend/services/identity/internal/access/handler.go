// Package access provides HTTP handlers for administrative access, invitations, and session resolution.
package access

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/accounts"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/sessions"
	"github.com/gin-gonic/gin"
)

// Handler handles administrative and internal identity endpoints.
type Handler struct {
	invitations  Store
	accounts     accounts.Store
	sessions     sessions.Store
	serviceToken string
}

// NewHandler initializes an access Handler.
func NewHandler(invitations Store, accs accounts.Store, sess sessions.Store, serviceToken string) *Handler {
	if serviceToken == "" {
		serviceToken = "identity-service-secret"
	}
	return &Handler{
		invitations:  invitations,
		accounts:     accs,
		sessions:     sess,
		serviceToken: serviceToken,
	}
}

func (h *Handler) requireAdmin(c *gin.Context) (string, error) {
	// Service token can act with administrative authority
	if h.serviceToken != "" && c.GetHeader("X-Service-Token") == h.serviceToken {
		return "system_admin", nil
	}

	cookie, err := c.Cookie(sessions.SessionCookieName)
	if err != nil || cookie == "" {
		return "", errors.New("unauthorized")
	}

	sess, err := h.sessions.GetSession(c.Request.Context(), cookie)
	if err != nil {
		return "", errors.New("unauthorized")
	}

	acc, err := h.accounts.GetAccountByID(c.Request.Context(), sess.UserID)
	if err != nil || acc.Status == "suspended" {
		return "", errors.New("forbidden")
	}

	for _, role := range acc.Roles {
		if role == "admin" {
			return acc.ID, nil
		}
	}

	return "", errors.New("forbidden")
}

// InviteInstructor godoc
// @Summary Invite an instructor
// @Description Sends an invitation for an instructor role to the specified email (admin only).
// @Tags access
// @Accept json
// @Produce json
// @Param body body InviteInstructorRequest true "Invitation parameters"
// @Success 200 {object} InviteInstructorResponse
// @Failure 400 {object} accounts.ErrorResponse
// @Failure 401 {object} accounts.ErrorResponse
// @Failure 403 {object} accounts.ErrorResponse
// @Router /api/identity/instructor-invitations [post]
// @id identityInviteInstructor
func (h *Handler) InviteInstructor(c *gin.Context) {
	adminID, err := h.requireAdmin(c)
	if err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{
				Error:   "unauthorized",
				Message: "Authentication required",
			})
			return
		}
		c.JSON(http.StatusForbidden, accounts.ErrorResponse{
			Error:   "forbidden",
			Message: "Admin privileges required",
		})
		return
	}

	var req InviteInstructorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "invalid_request",
			Message: "Valid email is required",
		})
		return
	}

	inv, token, err := h.invitations.CreateInvitation(c.Request.Context(), req.Email, adminID, 7*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, accounts.ErrorResponse{
			Error:   "invitation_failed",
			Message: "Failed to create invitation",
		})
		return
	}

	c.JSON(http.StatusOK, InviteInstructorResponse{
		InviteID:        inv.ID,
		Status:          inv.Status,
		InvitationToken: token,
	})
}

// AcceptInstructorInvitation godoc
// @Summary Accept instructor invitation
// @Description Accepts an instructor invitation with token, granting instructor role to the logged-in user.
// @Tags access
// @Accept json
// @Produce json
// @Param body body AcceptInvitationRequest true "Accept invitation payload"
// @Success 200 {object} AcceptInvitationResponse
// @Failure 400 {object} accounts.ErrorResponse
// @Failure 401 {object} accounts.ErrorResponse
// @Failure 403 {object} accounts.ErrorResponse
// @Router /api/identity/instructor-invitations/accept [post]
// @id identityAcceptInstructorInvitation
func (h *Handler) AcceptInstructorInvitation(c *gin.Context) {
	cookie, err := c.Cookie(sessions.SessionCookieName)
	if err != nil || cookie == "" {
		c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{
			Error:   "unauthorized",
			Message: "Login required to accept instructor invitation",
		})
		return
	}

	sess, err := h.sessions.GetSession(c.Request.Context(), cookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{
			Error:   "session_invalid",
			Message: "Active session required",
		})
		return
	}

	acc, err := h.accounts.GetAccountByID(c.Request.Context(), sess.UserID)
	if err != nil || acc.Status == "suspended" {
		c.JSON(http.StatusForbidden, accounts.ErrorResponse{
			Error:   "forbidden",
			Message: "Account unavailable",
		})
		return
	}

	var req AcceptInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "invalid_request",
			Message: "Invitation token is required",
		})
		return
	}

	inv, err := h.invitations.GetInvitationByToken(c.Request.Context(), req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "invalid_invitation",
			Message: "Invitation not found",
		})
		return
	}

	if accounts.NormalizeEmail(acc.Email) != accounts.NormalizeEmail(inv.Email) {
		c.JSON(http.StatusForbidden, accounts.ErrorResponse{
			Error:   "email_mismatch",
			Message: "Invitation was issued for a different email address",
		})
		return
	}

	if _, err := h.invitations.AcceptInvitation(c.Request.Context(), req.Token); err != nil {
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "invitation_error",
			Message: err.Error(),
		})
		return
	}

	updatedAcc, err := h.accounts.AddRole(c.Request.Context(), acc.ID, "instructor")
	if err != nil {
		c.JSON(http.StatusInternalServerError, accounts.ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to grant role",
		})
		return
	}

	c.JSON(http.StatusOK, AcceptInvitationResponse{
		Roles: updatedAcc.Roles,
	})
}

// CancelInstructorInvitation godoc
// @Summary Cancel instructor invitation
// @Description Cancels a pending instructor invitation (admin only).
// @Tags access
// @Produce json
// @Param inviteId path string true "Invitation ID"
// @Param expectedVersion query int true "Expected version"
// @Success 200 {object} CancelInvitationResponse
// @Failure 401 {object} accounts.ErrorResponse
// @Failure 403 {object} accounts.ErrorResponse
// @Failure 409 {object} accounts.ErrorResponse
// @Router /api/identity/instructor-invitations/{inviteId} [delete]
// @id identityCancelInstructorInvitation
func (h *Handler) CancelInstructorInvitation(c *gin.Context) {
	if _, err := h.requireAdmin(c); err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{Error: "unauthorized", Message: "Login required"})
			return
		}
		c.JSON(http.StatusForbidden, accounts.ErrorResponse{Error: "forbidden", Message: "Admin privileges required"})
		return
	}

	inviteID := c.Param("inviteId")
	expectedVersionStr := c.Query("expectedVersion")
	expectedVersion, err := strconv.Atoi(expectedVersionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "invalid_version",
			Message: "expectedVersion query parameter must be an integer",
		})
		return
	}

	inv, err := h.invitations.CancelInvitation(c.Request.Context(), inviteID, expectedVersion)
	if err != nil {
		if errors.Is(err, ErrVersionConflict) {
			c.JSON(http.StatusConflict, accounts.ErrorResponse{
				Error:   "version_conflict",
				Message: "Invitation version conflict",
			})
			return
		}
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "cancellation_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, CancelInvitationResponse{
		Status:  inv.Status,
		Version: inv.Version,
	})
}

// SetAccountStatus godoc
// @Summary Set account status
// @Description Suspends or reactivates an account (admin only).
// @Tags access
// @Accept json
// @Produce json
// @Param userId path string true "User ID"
// @Param body body SetAccountStatusRequest true "Status update payload"
// @Success 200 {object} SetAccountStatusResponse
// @Failure 400 {object} accounts.ErrorResponse
// @Failure 401 {object} accounts.ErrorResponse
// @Failure 403 {object} accounts.ErrorResponse
// @Failure 409 {object} accounts.ErrorResponse
// @Router /api/identity/accounts/{userId}/status [patch]
// @id identitySetAccountStatus
func (h *Handler) SetAccountStatus(c *gin.Context) {
	if _, err := h.requireAdmin(c); err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{Error: "unauthorized", Message: "Login required"})
			return
		}
		c.JSON(http.StatusForbidden, accounts.ErrorResponse{Error: "forbidden", Message: "Admin privileges required"})
		return
	}

	userID := c.Param("userId")
	var req SetAccountStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "invalid_request",
			Message: "Status, expectedVersion, and reason are required",
		})
		return
	}

	acc, err := h.accounts.SetAccountStatus(c.Request.Context(), userID, req.Status, req.ExpectedVersion)
	if err != nil {
		if errors.Is(err, accounts.ErrVersionConflict) {
			c.JSON(http.StatusConflict, accounts.ErrorResponse{
				Error:   "version_conflict",
				Message: "Account version conflict",
			})
			return
		}
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "update_failed",
			Message: err.Error(),
		})
		return
	}

	if req.Status == "suspended" {
		_ = h.sessions.RevokeAllForUser(c.Request.Context(), userID)
	}

	c.JSON(http.StatusOK, SetAccountStatusResponse{
		AccountStatus:  acc.Status,
		SessionVersion: acc.Version,
	})
}

// RevokeInstructor godoc
// @Summary Revoke instructor role
// @Description Removes the instructor qualification from an account (admin only).
// @Tags access
// @Produce json
// @Param userId path string true "User ID"
// @Param expectedVersion query int true "Expected version"
// @Param reason query string false "Reason"
// @Success 200 {object} RevokeInstructorResponse
// @Failure 400 {object} accounts.ErrorResponse
// @Failure 401 {object} accounts.ErrorResponse
// @Failure 403 {object} accounts.ErrorResponse
// @Failure 409 {object} accounts.ErrorResponse
// @Router /api/identity/accounts/{userId}/instructor-role [delete]
// @id identityRevokeInstructor
func (h *Handler) RevokeInstructor(c *gin.Context) {
	if _, err := h.requireAdmin(c); err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{Error: "unauthorized", Message: "Login required"})
			return
		}
		c.JSON(http.StatusForbidden, accounts.ErrorResponse{Error: "forbidden", Message: "Admin privileges required"})
		return
	}

	userID := c.Param("userId")
	expectedVersionStr := c.Query("expectedVersion")
	expectedVersion, err := strconv.Atoi(expectedVersionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "invalid_version",
			Message: "expectedVersion query parameter must be an integer",
		})
		return
	}

	acc, err := h.accounts.RevokeRole(c.Request.Context(), userID, "instructor", expectedVersion)
	if err != nil {
		if errors.Is(err, accounts.ErrVersionConflict) {
			c.JSON(http.StatusConflict, accounts.ErrorResponse{
				Error:   "version_conflict",
				Message: "Account version conflict",
			})
			return
		}
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "revoke_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, RevokeInstructorResponse{
		Roles:          acc.Roles,
		AccountVersion: acc.Version,
	})
}

// ResolveSession godoc
// @Summary Resolve session for internal service callers
// @Description Resolves an opaque session credential to user identity, roles, and status. Requires X-Service-Token header.
// @Tags access
// @Accept json
// @Produce json
// @Param X-Service-Token header string true "Service authentication token"
// @Param body body ResolveSessionRequest true "Session resolve payload"
// @Success 200 {object} ResolveSessionResponse
// @Failure 401 {object} accounts.ErrorResponse
// @Failure 403 {object} accounts.ErrorResponse
// @Router /internal/identity/sessions/resolve [post]
// @id identityResolveSession
func (h *Handler) ResolveSession(c *gin.Context) {
	token := c.GetHeader("X-Service-Token")
	if token == "" || token != h.serviceToken {
		c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{
			Error:   "unauthorized_service",
			Message: "Valid service credentials required",
		})
		return
	}

	var req ResolveSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, accounts.ErrorResponse{
			Error:   "invalid_request",
			Message: "Opaque session token is required",
		})
		return
	}

	sess, err := h.sessions.GetSession(c.Request.Context(), req.OpaqueSession)
	if err != nil {
		c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{
			Error:   "session_invalid",
			Message: "Session expired or revoked",
		})
		return
	}

	acc, err := h.accounts.GetAccountByID(c.Request.Context(), sess.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, accounts.ErrorResponse{
			Error:   "account_not_found",
			Message: "Account does not exist",
		})
		return
	}

	if acc.Status == "suspended" {
		c.JSON(http.StatusForbidden, accounts.ErrorResponse{
			Error:   "account_suspended",
			Message: "Account has been suspended",
		})
		return
	}

	c.JSON(http.StatusOK, ResolveSessionResponse{
		UserID:         acc.ID,
		Email:          acc.Email,
		EmailVerified:  acc.EmailVerified,
		Roles:          acc.Roles,
		AccountStatus:  acc.Status,
		SessionVersion: sess.Version,
	})
}
