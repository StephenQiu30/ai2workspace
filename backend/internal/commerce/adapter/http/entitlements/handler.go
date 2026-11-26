package entitlements

import (
	"net/http"

	authhttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/auth"
	entitlementsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/entitlements"
	entitlementsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/entitlements"
	"github.com/gin-gonic/gin"
)

// Handler manages entitlement HTTP endpoints.
type Handler struct {
	store    entitlementsapp.Store
	resolver authhttp.Resolver
}

// NewHandler creates an entitlements Handler.
func NewHandler(store entitlementsapp.Store, resolver authhttp.Resolver) *Handler {
	return &Handler{
		store:    store,
		resolver: resolver,
	}
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
	principal, ok := authhttp.ResolvePrincipal(c, h.resolver)
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

func entitlementToResponse(e *entitlementsdomain.Entitlement) EntitlementResponse {
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
