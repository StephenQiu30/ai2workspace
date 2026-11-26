package access

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	accountshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/accounts"
	accessapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/access"
	accountsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/accounts"
	sessionsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/sessions"
	accessdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/access"
	accountsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/accounts"
	"github.com/gin-gonic/gin"
)

// Options configures optional behavior for access Handler.
type Options struct {
	DevExposeTokens bool
	Mailer          accountsapp.Mailer
}

// Handler handles administrative and internal identity endpoints.
type Handler struct {
	invitations accessapp.Store
	accounts    accountsapp.Store
	sessions    sessionsapp.Store

	devExposeTokens bool
	mailer          accountsapp.Mailer
}

// NewHandlerWithOptions initializes an access Handler with explicit options.
func NewHandlerWithOptions(invitations accessapp.Store, accs accountsapp.Store, sess sessionsapp.Store, opts Options) *Handler {
	return &Handler{
		invitations: invitations,
		accounts:    accs,
		sessions:    sess,

		devExposeTokens: opts.DevExposeTokens,
		mailer:          opts.Mailer,
	}
}

func (h *Handler) requireAdmin(c *gin.Context) (string, error) {
	cookie, err := c.Cookie(sessionsapp.SessionCookieName)
	if err != nil || cookie == "" {
		return "", errors.New("unauthorized")
	}

	sess, err := h.sessions.GetSession(c.Request.Context(), cookie)
	if err != nil {
		return "", errors.New("unauthorized")
	}

	acc, err := h.accounts.GetAccountByID(c.Request.Context(), sess.UserID)
	if err != nil || acc.Status == accountsdomain.StatusSuspended {
		return "", errors.New("forbidden")
	}

	for _, role := range acc.Roles {
		if role == "admin" {
			return acc.ID, nil
		}
	}

	return "", errors.New("forbidden")
}

func parseOptionalInt(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 0, errors.New("must be a positive integer")
	}
	return v, nil
}

// ListAccounts godoc
// @Summary List accounts (admin only)
// @Description Returns a paginated list of accounts ordered by creation time descending, supporting status, role, and search query filters.
// @Tags access
// @Produce json
// @Param status query string false "Filter by account status (active or suspended)"
// @Param role query string false "Filter by role (student, instructor, admin)"
// @Param q query string false "Search by email, nickname, or user ID"
// @Param page query int false "Page number (default 1)"
// @Param pageSize query int false "Page size (default 20, max 100)"
// @Success 200 {object} ListAccountsResponse
// @Failure 400 {object} accountshttp.ErrorResponse
// @Failure 401 {object} accountshttp.ErrorResponse
// @Failure 403 {object} accountshttp.ErrorResponse
// @Router /api/identity/accounts [get]
// @ID identityListAccounts
func (h *Handler) ListAccounts(c *gin.Context) {
	if _, err := h.requireAdmin(c); err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{
				Error:   "unauthorized",
				Message: "Authentication required",
			})
			return
		}
		c.JSON(http.StatusForbidden, accountshttp.ErrorResponse{
			Error:   "forbidden",
			Message: "Admin privileges required",
		})
		return
	}

	page, err := parseOptionalInt(c.Query("page"), 1)
	if err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_pagination",
			Message: "page must be a positive integer",
		})
		return
	}
	pageSize, err := parseOptionalInt(c.Query("pageSize"), 20)
	if err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_pagination",
			Message: "pageSize must be a positive integer",
		})
		return
	}
	if pageSize > 100 {
		pageSize = 100
	}

	statusFilter := strings.TrimSpace(c.Query("status"))
	if statusFilter != "" {
		if err := accountsdomain.ValidateStatus(statusFilter); err != nil {
			c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
				Error:   "invalid_status",
				Message: "status must be active or suspended",
			})
			return
		}
	}

	accs, total, err := h.accounts.ListAccounts(c.Request.Context(), accountsdomain.ListAccountsFilter{
		Status:   statusFilter,
		Role:     strings.TrimSpace(c.Query("role")),
		Query:    strings.TrimSpace(c.Query("q")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, accountshttp.ErrorResponse{
			Error:   "list_failed",
			Message: "Failed to list accounts",
		})
		return
	}

	items := make([]AccountSummary, 0, len(accs))
	for _, acc := range accs {
		items = append(items, AccountSummary{
			ID:            acc.ID,
			Email:         acc.Email,
			Nickname:      acc.Nickname,
			EmailVerified: acc.EmailVerified,
			Status:        acc.Status,
			Roles:         acc.Roles,
			Version:       acc.Version,
			CreatedAt:     acc.CreatedAt,
			UpdatedAt:     acc.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, ListAccountsResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// ListInstructorInvitations godoc
// @Summary List instructor invitations (admin only)
// @Description Returns instructor invitations ordered by creation time descending.
// @Tags access
// @Produce json
// @Param status query string false "Filter by invitation status (pending, accepted, cancelled, expired)"
// @Param page query int false "Page number (default 1)"
// @Param pageSize query int false "Page size (default 50, max 100)"
// @Success 200 {object} ListInvitationsResponse
// @Failure 400 {object} accountshttp.ErrorResponse
// @Failure 401 {object} accountshttp.ErrorResponse
// @Failure 403 {object} accountshttp.ErrorResponse
// @Router /api/identity/instructor-invitations [get]
// @ID identityListInstructorInvitations
func (h *Handler) ListInstructorInvitations(c *gin.Context) {
	if _, err := h.requireAdmin(c); err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{
				Error:   "unauthorized",
				Message: "Authentication required",
			})
			return
		}
		c.JSON(http.StatusForbidden, accountshttp.ErrorResponse{
			Error:   "forbidden",
			Message: "Admin privileges required",
		})
		return
	}

	page, err := parseOptionalInt(c.Query("page"), 1)
	if err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_pagination",
			Message: "page must be a positive integer",
		})
		return
	}
	pageSize, err := parseOptionalInt(c.Query("pageSize"), 50)
	if err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_pagination",
			Message: "pageSize must be a positive integer",
		})
		return
	}
	if pageSize > 100 {
		pageSize = 100
	}

	invs, total, err := h.invitations.ListInvitations(c.Request.Context(), accessdomain.ListInvitationsFilter{
		Status:   strings.TrimSpace(c.Query("status")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, accountshttp.ErrorResponse{
			Error:   "list_failed",
			Message: "Failed to list instructor invitations",
		})
		return
	}

	items := make([]accessdomain.Invitation, 0, len(invs))
	for _, inv := range invs {
		items = append(items, *inv)
	}

	c.JSON(http.StatusOK, ListInvitationsResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// InviteInstructor godoc
// @Summary Invite an instructor
// @Description Sends an invitation for an instructor role to the specified email (admin only).
// @Tags access
// @Accept json
// @Produce json
// @Param body body InviteInstructorRequest true "Invitation parameters"
// @Success 200 {object} InviteInstructorResponse
// @Failure 400 {object} accountshttp.ErrorResponse
// @Failure 401 {object} accountshttp.ErrorResponse
// @Failure 403 {object} accountshttp.ErrorResponse
// @Failure 503 {object} accountshttp.ErrorResponse
// @Router /api/identity/instructor-invitations [post]
// @ID identityInviteInstructor
func (h *Handler) InviteInstructor(c *gin.Context) {
	adminID, err := h.requireAdmin(c)
	if err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{
				Error:   "unauthorized",
				Message: "Authentication required",
			})
			return
		}
		c.JSON(http.StatusForbidden, accountshttp.ErrorResponse{
			Error:   "forbidden",
			Message: "Admin privileges required",
		})
		return
	}

	var req InviteInstructorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_request",
			Message: "Valid email is required",
		})
		return
	}

	inv, token, err := h.invitations.CreateInvitation(c.Request.Context(), req.Email, adminID, 7*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, accountshttp.ErrorResponse{
			Error:   "invitation_failed",
			Message: "Failed to create invitation",
		})
		return
	}

	_ = h.accounts.RecordMailTask(c.Request.Context(), accountsdomain.MailTask{
		Recipient: inv.Email,
		Subject:   "Instructor invitation",
		Purpose:   accountsdomain.PurposeInstructorInvite,
		Token:     token,
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	})

	if h.mailer != nil {
		body := fmt.Sprintf("You have been invited as an instructor on ai2workspace. Use this invitation token after signing in with your verified email: %s", token)
		if err := h.mailer.Send(c.Request.Context(), accountsapp.MailMessage{
			To:      inv.Email,
			Subject: "Instructor invitation",
			Body:    body,
		}); err != nil {
			_ = h.accounts.UpdateMailTaskStatus(c.Request.Context(), inv.Email, accountsdomain.PurposeInstructorInvite, token, "failed")
			c.JSON(http.StatusServiceUnavailable, accountshttp.ErrorResponse{
				Error:   "mail_delivery_failed",
				Message: "Invitation created, but email delivery failed. Please retry later.",
			})
			return
		}
	}
	_ = h.accounts.UpdateMailTaskStatus(c.Request.Context(), inv.Email, accountsdomain.PurposeInstructorInvite, token, "sent")

	_ = h.invitations.RecordAuditLog(c.Request.Context(), accessdomain.AuditLog{
		ActorID:   adminID,
		TargetID:  inv.ID,
		Action:    "invite_instructor",
		Detail:    inv.Email,
		CreatedAt: time.Now().UTC(),
	})

	resp := InviteInstructorResponse{
		InviteID: inv.ID,
		Status:   inv.Status,
	}
	if h.devExposeTokens {
		resp.InvitationToken = token
	}
	c.JSON(http.StatusOK, resp)
}

// AcceptInstructorInvitation godoc
// @Summary Accept instructor invitation
// @Description Accepts an instructor invitation with token, granting instructor role to the logged-in verified user matching the invitation email.
// @Tags access
// @Accept json
// @Produce json
// @Param body body AcceptInvitationRequest true "Accept invitation payload"
// @Success 200 {object} AcceptInvitationResponse
// @Failure 400 {object} accountshttp.ErrorResponse
// @Failure 401 {object} accountshttp.ErrorResponse
// @Failure 403 {object} accountshttp.ErrorResponse
// @Router /api/identity/instructor-invitations/accept [post]
// @ID identityAcceptInstructorInvitation
func (h *Handler) AcceptInstructorInvitation(c *gin.Context) {
	cookie, err := c.Cookie(sessionsapp.SessionCookieName)
	if err != nil || cookie == "" {
		c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{
			Error:   "unauthorized",
			Message: "Login required to accept instructor invitation",
		})
		return
	}

	sess, err := h.sessions.GetSession(c.Request.Context(), cookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{
			Error:   "session_invalid",
			Message: "Active session required",
		})
		return
	}

	acc, err := h.accounts.GetAccountByID(c.Request.Context(), sess.UserID)
	if err != nil || acc.Status == accountsdomain.StatusSuspended {
		c.JSON(http.StatusForbidden, accountshttp.ErrorResponse{
			Error:   "forbidden",
			Message: "Account unavailable",
		})
		return
	}

	if !acc.EmailVerified {
		c.JSON(http.StatusForbidden, accountshttp.ErrorResponse{
			Error:   "email_not_verified",
			Message: "Verified email is required to accept instructor invitation",
		})
		return
	}

	var req AcceptInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_request",
			Message: "Invitation token is required",
		})
		return
	}

	inv, err := h.invitations.GetInvitationByToken(c.Request.Context(), req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_invitation",
			Message: "Invitation not found",
		})
		return
	}

	if accountsdomain.NormalizeEmail(acc.Email) != accountsdomain.NormalizeEmail(inv.Email) {
		c.JSON(http.StatusForbidden, accountshttp.ErrorResponse{
			Error:   "email_mismatch",
			Message: "Invitation was issued for a different email address",
		})
		return
	}

	if _, err := h.invitations.AcceptInvitation(c.Request.Context(), req.Token); err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invitation_error",
			Message: err.Error(),
		})
		return
	}

	updatedAcc, err := h.accounts.AddRole(c.Request.Context(), acc.ID, "instructor")
	if err != nil {
		c.JSON(http.StatusInternalServerError, accountshttp.ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to grant role",
		})
		return
	}

	_ = h.invitations.RecordAuditLog(c.Request.Context(), accessdomain.AuditLog{
		ActorID:   acc.ID,
		TargetID:  inv.ID,
		Action:    "accept_instructor_invitation",
		Detail:    acc.Email,
		CreatedAt: time.Now().UTC(),
	})

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
// @Failure 400 {object} accountshttp.ErrorResponse
// @Failure 401 {object} accountshttp.ErrorResponse
// @Failure 403 {object} accountshttp.ErrorResponse
// @Failure 409 {object} accountshttp.ErrorResponse
// @Router /api/identity/instructor-invitations/{inviteId} [delete]
// @ID identityCancelInstructorInvitation
func (h *Handler) CancelInstructorInvitation(c *gin.Context) {
	adminID, err := h.requireAdmin(c)
	if err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{Error: "unauthorized", Message: "Login required"})
			return
		}
		c.JSON(http.StatusForbidden, accountshttp.ErrorResponse{Error: "forbidden", Message: "Admin privileges required"})
		return
	}

	inviteID := c.Param("inviteId")
	expectedVersionStr := c.Query("expectedVersion")
	expectedVersion, err := strconv.Atoi(expectedVersionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_version",
			Message: "expectedVersion query parameter must be an integer",
		})
		return
	}

	inv, err := h.invitations.CancelInvitation(c.Request.Context(), inviteID, expectedVersion)
	if err != nil {
		if errors.Is(err, accessdomain.ErrVersionConflict) {
			c.JSON(http.StatusConflict, accountshttp.ErrorResponse{
				Error:   "version_conflict",
				Message: "Invitation version conflict",
			})
			return
		}
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "cancellation_failed",
			Message: err.Error(),
		})
		return
	}

	_ = h.invitations.RecordAuditLog(c.Request.Context(), accessdomain.AuditLog{
		ActorID:   adminID,
		TargetID:  inv.ID,
		Action:    "cancel_instructor_invitation",
		Detail:    inv.Email,
		CreatedAt: time.Now().UTC(),
	})

	c.JSON(http.StatusOK, CancelInvitationResponse{
		Status:  inv.Status,
		Version: inv.Version,
	})
}

// SetAccountStatus godoc
// @Summary Set account status
// @Description Suspends or reactivates an account (admin only). Status must be active or suspended.
// @Tags access
// @Accept json
// @Produce json
// @Param userId path string true "User ID"
// @Param body body SetAccountStatusRequest true "Status update payload"
// @Success 200 {object} SetAccountStatusResponse
// @Failure 400 {object} accountshttp.ErrorResponse
// @Failure 401 {object} accountshttp.ErrorResponse
// @Failure 403 {object} accountshttp.ErrorResponse
// @Failure 404 {object} accountshttp.ErrorResponse
// @Failure 409 {object} accountshttp.ErrorResponse
// @Router /api/identity/accounts/{userId}/status [patch]
// @ID identitySetAccountStatus
func (h *Handler) SetAccountStatus(c *gin.Context) {
	adminID, err := h.requireAdmin(c)
	if err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{Error: "unauthorized", Message: "Login required"})
			return
		}
		c.JSON(http.StatusForbidden, accountshttp.ErrorResponse{Error: "forbidden", Message: "Admin privileges required"})
		return
	}

	userID := c.Param("userId")
	var req SetAccountStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_request",
			Message: "Status (active or suspended), expectedVersion, and reason are required",
		})
		return
	}

	if err := accountsdomain.ValidateStatus(req.Status); err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_status",
			Message: "Status must be either active or suspended",
		})
		return
	}

	acc, err := h.accounts.SetAccountStatus(c.Request.Context(), userID, req.Status, req.ExpectedVersion)
	if err != nil {
		if errors.Is(err, accountsdomain.ErrVersionConflict) {
			c.JSON(http.StatusConflict, accountshttp.ErrorResponse{
				Error:   "version_conflict",
				Message: "Account version conflict",
			})
			return
		}
		if errors.Is(err, accountsdomain.ErrAccountNotFound) {
			c.JSON(http.StatusNotFound, accountshttp.ErrorResponse{
				Error:   "account_not_found",
				Message: "Account not found",
			})
			return
		}
		if errors.Is(err, accountsdomain.ErrInvalidStatus) {
			c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
				Error:   "invalid_status",
				Message: "Status must be either active or suspended",
			})
			return
		}
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "update_failed",
			Message: err.Error(),
		})
		return
	}

	if req.Status == accountsdomain.StatusSuspended {
		_ = h.sessions.RevokeAllForUser(c.Request.Context(), userID)
	}

	_ = h.invitations.RecordAuditLog(c.Request.Context(), accessdomain.AuditLog{
		ActorID:   adminID,
		TargetID:  userID,
		Action:    "set_account_status",
		Detail:    req.Status,
		Reason:    strings.TrimSpace(req.Reason),
		CreatedAt: time.Now().UTC(),
	})

	c.JSON(http.StatusOK, SetAccountStatusResponse{
		AccountStatus:  acc.Status,
		SessionVersion: acc.Version,
	})
}

// RevokeInstructor godoc
// @Summary Revoke instructor role
// @Description Removes the instructor qualification from an account (admin only) and records an audit entry.
// @Tags access
// @Produce json
// @Param userId path string true "User ID"
// @Param expectedVersion query int true "Expected version"
// @Param reason query string false "Reason"
// @Success 200 {object} RevokeInstructorResponse
// @Failure 400 {object} accountshttp.ErrorResponse
// @Failure 401 {object} accountshttp.ErrorResponse
// @Failure 403 {object} accountshttp.ErrorResponse
// @Failure 404 {object} accountshttp.ErrorResponse
// @Failure 409 {object} accountshttp.ErrorResponse
// @Router /api/identity/accounts/{userId}/instructor-role [delete]
// @ID identityRevokeInstructor
func (h *Handler) RevokeInstructor(c *gin.Context) {
	adminID, err := h.requireAdmin(c)
	if err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusUnauthorized, accountshttp.ErrorResponse{Error: "unauthorized", Message: "Login required"})
			return
		}
		c.JSON(http.StatusForbidden, accountshttp.ErrorResponse{Error: "forbidden", Message: "Admin privileges required"})
		return
	}

	userID := c.Param("userId")
	expectedVersionStr := c.Query("expectedVersion")
	expectedVersion, err := strconv.Atoi(expectedVersionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "invalid_version",
			Message: "expectedVersion query parameter must be an integer",
		})
		return
	}
	reason := strings.TrimSpace(c.Query("reason"))

	acc, err := h.accounts.RevokeRole(c.Request.Context(), userID, "instructor", expectedVersion)
	if err != nil {
		if errors.Is(err, accountsdomain.ErrVersionConflict) {
			c.JSON(http.StatusConflict, accountshttp.ErrorResponse{
				Error:   "version_conflict",
				Message: "Account version conflict",
			})
			return
		}
		if errors.Is(err, accountsdomain.ErrAccountNotFound) {
			c.JSON(http.StatusNotFound, accountshttp.ErrorResponse{
				Error:   "account_not_found",
				Message: "Account not found",
			})
			return
		}
		c.JSON(http.StatusBadRequest, accountshttp.ErrorResponse{
			Error:   "revoke_failed",
			Message: err.Error(),
		})
		return
	}

	_ = h.invitations.RecordAuditLog(c.Request.Context(), accessdomain.AuditLog{
		ActorID:   adminID,
		TargetID:  userID,
		Action:    "revoke_instructor_role",
		Detail:    "instructor",
		Reason:    reason,
		CreatedAt: time.Now().UTC(),
	})

	c.JSON(http.StatusOK, RevokeInstructorResponse{
		Roles:          acc.Roles,
		AccountVersion: acc.Version,
	})
}
