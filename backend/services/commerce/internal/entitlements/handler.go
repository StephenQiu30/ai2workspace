package entitlements

import (
	"net/http"
	"strings"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/auth"
	"github.com/gin-gonic/gin"
)

// CheckEntitlementRequest represents internal entitlement check payload.
type CheckEntitlementRequest struct {
	UserID   string `json:"userId" binding:"required"`
	CourseID string `json:"courseId" binding:"required"`
	Purpose  string `json:"purpose"` // "playback", "discussion", "knowledge"
}

// CheckEntitlementResponse represents internal entitlement check output.
type CheckEntitlementResponse struct {
	Allowed      bool   `json:"allowed"`
	GrantVersion int    `json:"grantVersion"`
	Reason       string `json:"reason"`
}

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

// Handler manages entitlement HTTP endpoints.
type Handler struct {
	store        Store
	resolver     auth.Resolver
	serviceToken string
}

// NewHandler creates an entitlements Handler.
func NewHandler(store Store, resolver auth.Resolver, serviceToken string) *Handler {
	return &Handler{
		store:        store,
		resolver:     resolver,
		serviceToken: serviceToken,
	}
}

// CheckEntitlement verifies whether a student holds a valid purchase grant.
// @ID commerceCheckEntitlement
// @Summary Check user course entitlement (Internal)
// @Description Internal endpoint consumed by learning, community, or assistant to verify access. Requires X-Service-Token header.
// @Tags Commerce
// @Accept json
// @Produce json
// @Param X-Service-Token header string true "Service authentication token"
// @Param request body CheckEntitlementRequest true "Entitlement check payload"
// @Success 200 {object} CheckEntitlementResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /internal/commerce/entitlements/check [post]
func (h *Handler) CheckEntitlement(c *gin.Context) {
	if !auth.VerifyServiceToken(c, h.serviceToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized_service"})
		return
	}

	var req CheckEntitlementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	allowed, grantVersion, reason, err := h.store.CheckEntitlement(c.Request.Context(), req.UserID, req.CourseID, req.Purpose)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, CheckEntitlementResponse{
		Allowed:      allowed,
		GrantVersion: grantVersion,
		Reason:       reason,
	})
}

// ListUserEntitlementsInternal returns entitlements for a given user for trusted internal callers.
// @ID commerceListUserEntitlements
// @Summary List user entitlements (Internal)
// @Description Internal endpoint consumed by learning to aggregate purchased courses. Requires X-Service-Token header.
// @Tags Commerce
// @Produce json
// @Param X-Service-Token header string true "Service authentication token"
// @Param userId path string true "User ID"
// @Param status query string false "Optional status filter (active or revoked)"
// @Success 200 {array} EntitlementResponse
// @Failure 401 {object} map[string]string
// @Router /internal/commerce/users/{userId}/entitlements [get]
func (h *Handler) ListUserEntitlementsInternal(c *gin.Context) {
	if !auth.VerifyServiceToken(c, h.serviceToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized_service"})
		return
	}

	userID := strings.TrimSpace(c.Param("userId"))
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId is required"})
		return
	}

	statusFilter := strings.TrimSpace(c.Query("status"))
	list, err := h.store.ListUserEntitlements(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := make([]EntitlementResponse, 0, len(list))
	for _, e := range list {
		if statusFilter != "" && string(e.Status) != statusFilter {
			continue
		}
		resp = append(resp, entitlementToResponse(e))
	}

	c.JSON(http.StatusOK, resp)
}

// ListMyEntitlements returns all active and revoked grants for the current authenticated user.
// @ID commerceListMyEntitlements
// @Summary List current user entitlements
// @Tags Commerce
// @Produce json
// @Success 200 {array} EntitlementResponse
// @Failure 401 {object} map[string]string
// @Router /api/commerce/me/entitlements [get]
func (h *Handler) ListMyEntitlements(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}

	list, err := h.store.ListUserEntitlements(c.Request.Context(), principal.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := make([]EntitlementResponse, 0, len(list))
	for _, e := range list {
		resp = append(resp, entitlementToResponse(e))
	}

	c.JSON(http.StatusOK, resp)
}

func entitlementToResponse(e *Entitlement) EntitlementResponse {
	return EntitlementResponse{
		ID:           e.ID,
		UserID:       e.UserID,
		CourseID:     e.CourseID,
		OrderID:      e.OrderID,
		Status:       string(e.Status),
		GrantVersion: e.GrantVersion,
		ActivatedAt:  e.ActivatedAt,
		RevokedAt:    e.RevokedAt,
	}
}
