package refunds

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ApplyRefundRequest defines payload to request refund for an order.
type ApplyRefundRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// ReviewRefundRequest defines admin decision payload for refund application.
type ReviewRefundRequest struct {
	Decision        string `json:"decision" binding:"required"` // "approved", "rejected"
	Reason          string `json:"reason"`
	ExpectedVersion int    `json:"expectedVersion"`
}

// RefundResponse represents serialized refund application.
type RefundResponse struct {
	ID               string    `json:"id"`
	OrderID          string    `json:"orderId"`
	UserID           string    `json:"userId"`
	Reason           string    `json:"reason"`
	AmountMinor      int64     `json:"amountMinor"`
	ApplicationState string    `json:"applicationState"`
	ChannelState     string    `json:"channelState"`
	RejectionReason  string    `json:"rejectionReason,omitempty"`
	Version          int       `json:"version"`
	Deadline         time.Time `json:"deadline"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// RefundListResponse represents paginated refund list for admin console.
type RefundListResponse struct {
	Items []RefundResponse `json:"items"`
	Total int              `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

// Handler manages refund HTTP endpoints.
type Handler struct {
	store Store
}

// NewHandler creates a refunds Handler.
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// ApplyRefund submits a 7x24h refund request for a paid order.
// @ID commerceApplyRefund
// @Summary Apply for order refund
// @Description Allows full refund within 7x24h of actual course delivery.
// @Tags Commerce
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body ApplyRefundRequest true "Refund reason"
// @Success 201 {object} RefundResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/commerce/orders/{id}/refunds [post]
func (h *Handler) ApplyRefund(c *gin.Context) {
	orderID := c.Param("id")
	var req ApplyRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "mock_buyer"
	}

	app, err := h.store.ApplyRefund(c.Request.Context(), userID, orderID, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrRefundWindowExpired):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, refundToResponse(app))
}

// GetRefund returns the status and progress of a refund application.
// @ID commerceGetRefund
// @Summary Get refund application details
// @Tags Commerce
// @Produce json
// @Param id path string true "Refund ID"
// @Success 200 {object} RefundResponse
// @Failure 404 {object} map[string]string
// @Router /api/commerce/refunds/{id} [get]
func (h *Handler) GetRefund(c *gin.Context) {
	refID := c.Param("id")
	app, err := h.store.GetRefund(c.Request.Context(), refID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, refundToResponse(app))
}

// ReviewRefund executes admin approval or rejection of a refund.
// @ID commerceReviewRefund
// @Summary Review refund application (Admin)
// @Description Approving refund triggers gateway return and revokes user entitlement.
// @Tags Commerce
// @Accept json
// @Produce json
// @Param id path string true "Refund ID"
// @Param request body ReviewRefundRequest true "Review decision"
// @Success 200 {object} RefundResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/commerce/refunds/{id}/review [post]
func (h *Handler) ReviewRefund(c *gin.Context) {
	refID := c.Param("id")
	var req ReviewRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminID := c.GetHeader("X-User-ID")
	if adminID == "" {
		adminID = "admin_mock"
	}

	approved := req.Decision == "approved"
	app, err := h.store.ReviewRefund(c.Request.Context(), adminID, refID, approved, req.Reason, req.ExpectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, refundToResponse(app))
}

// ListRefunds returns paginated refund applications for admin review.
// @ID commerceListRefunds
// @Summary List refund applications (Admin)
// @Tags Commerce
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} RefundListResponse
// @Router /api/commerce/refunds [get]
func (h *Handler) ListRefunds(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	items, total, err := h.store.ListRefunds(c.Request.Context(), page, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := make([]RefundResponse, 0, len(items))
	for _, r := range items {
		resp = append(resp, refundToResponse(r))
	}

	c.JSON(http.StatusOK, RefundListResponse{
		Items: resp,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

func refundToResponse(r *RefundApplication) RefundResponse {
	return RefundResponse{
		ID:               r.ID,
		OrderID:          r.OrderID,
		UserID:           r.UserID,
		Reason:           r.Reason,
		AmountMinor:      r.AmountMinor,
		ApplicationState: string(r.ApplicationState),
		ChannelState:     string(r.ChannelState),
		RejectionReason:  r.RejectionReason,
		Version:          r.Version,
		Deadline:         r.Deadline,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}
