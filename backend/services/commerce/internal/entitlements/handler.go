package entitlements

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// CheckEntitlementRequest represents internal check payload.
type CheckEntitlementRequest struct {
	UserID   string `json:"userId" binding:"required"`
	CourseID string `json:"courseId" binding:"required"`
	Purpose  string `json:"purpose"` // "playback", "discussion", "knowledge"
}

// CheckEntitlementResponse represents internal check output.
type CheckEntitlementResponse struct {
	Allowed      bool   `json:"allowed"`
	GrantVersion int    `json:"grantVersion"`
	Reason       string `json:"reason"`
}

// EntitlementResponse represents serialized student grant.
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
	store Store
}

// NewHandler creates an entitlements Handler.
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// CheckEntitlement verifies whether a student holds valid purchase grant.
// @ID commerceCheckEntitlement
// @Summary Check user course entitlement (Internal)
// @Description Internal endpoint consumed by learning, community, or assistant to verify access.
// @Tags Commerce
// @Accept json
// @Produce json
// @Param request body CheckEntitlementRequest true "Entitlement check payload"
// @Success 200 {object} CheckEntitlementResponse
// @Failure 400 {object} map[string]string
// @Router /internal/commerce/entitlements/check [post]
func (h *Handler) CheckEntitlement(c *gin.Context) {
	var req CheckEntitlementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	allowed, grantVersion, reason, _ := h.store.CheckEntitlement(c.Request.Context(), req.UserID, req.CourseID, req.Purpose)
	c.JSON(http.StatusOK, CheckEntitlementResponse{
		Allowed:      allowed,
		GrantVersion: grantVersion,
		Reason:       reason,
	})
}

// ListMyEntitlements returns all active and revoked grants for current user.
// @ID commerceListMyEntitlements
// @Summary List current user entitlements
// @Tags Commerce
// @Produce json
// @Success 200 {array} EntitlementResponse
// @Router /api/commerce/me/entitlements [get]
func (h *Handler) ListMyEntitlements(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "mock_buyer"
	}

	list, err := h.store.ListUserEntitlements(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := make([]EntitlementResponse, 0, len(list))
	for _, e := range list {
		resp = append(resp, EntitlementResponse{
			ID:           e.ID,
			UserID:       e.UserID,
			CourseID:     e.CourseID,
			OrderID:      e.OrderID,
			Status:       string(e.Status),
			GrantVersion: e.GrantVersion,
			ActivatedAt:  e.ActivatedAt,
			RevokedAt:    e.RevokedAt,
		})
	}

	c.JSON(http.StatusOK, resp)
}
