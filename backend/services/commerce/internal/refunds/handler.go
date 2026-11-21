package refunds

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
	"github.com/gin-gonic/gin"
)

// ApplyRefundRequest defines payload to request a refund for a paid order.
type ApplyRefundRequest struct {
	Reason        string `json:"reason" binding:"required"`
	ExceptionType string `json:"exceptionType,omitempty"` // quality_issue, delivery_unavailable, duplicate_charge, legal_entitlement
}

// CancelRefundRequest defines optional optimistic lock payload for cancelling a pending refund.
type CancelRefundRequest struct {
	ExpectedVersion int `json:"expectedVersion"`
}

// ReviewRefundRequest defines admin decision payload for a refund application.
type ReviewRefundRequest struct {
	Decision        string `json:"decision" binding:"required"` // "approved" or "rejected"
	Reason          string `json:"reason"`
	ExpectedVersion int    `json:"expectedVersion"`
}

// ChannelConfirmRefundRequest defines channel callback or admin confirmation of refund execution result.
type ChannelConfirmRefundRequest struct {
	Status          string `json:"status" binding:"required"` // "success", "failed", "unknown"
	ChannelRefundID string `json:"channelRefundId,omitempty"`
	FailureReason   string `json:"failureReason,omitempty"`
	ExpectedVersion int    `json:"expectedVersion"`
	Signature       string `json:"signature,omitempty"`
}

// RefundResponse represents a serialized refund application.
type RefundResponse struct {
	ID                   string     `json:"id"`
	OrderID              string     `json:"orderId"`
	UserID               string     `json:"userId"`
	Reason               string     `json:"reason"`
	ExceptionType        string     `json:"exceptionType,omitempty"`
	IsOutOfWindow        bool       `json:"isOutOfWindow"`
	AmountMinor          int64      `json:"amountMinor"`
	ApplicationState     string     `json:"applicationState"`
	ChannelState         string     `json:"channelState"`
	ChannelRefundID      string     `json:"channelRefundId,omitempty"`
	ChannelFailureReason string     `json:"channelFailureReason,omitempty"`
	ReviewerID           string     `json:"reviewerId,omitempty"`
	ReviewedAt           *time.Time `json:"reviewedAt,omitempty"`
	RejectionReason      string     `json:"rejectionReason,omitempty"`
	Version              int        `json:"version"`
	FirstAvailableAt     *time.Time `json:"firstAvailableAt,omitempty"`
	Deadline             time.Time  `json:"deadline"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

// RefundListResponse represents a paginated refund list for admin console.
type RefundListResponse struct {
	Items []RefundResponse `json:"items"`
	Total int              `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

// Handler manages refund HTTP endpoints.
type Handler struct {
	store         Store
	orderStore    orders.Store
	resolver      auth.Resolver
	serviceToken  string
	webhookSecret string
}

// NewHandler creates a refunds Handler.
func NewHandler(store Store, orderStore orders.Store, resolver auth.Resolver, serviceToken, webhookSecret string) *Handler {
	return &Handler{
		store:         store,
		orderStore:    orderStore,
		resolver:      resolver,
		serviceToken:  serviceToken,
		webhookSecret: webhookSecret,
	}
}

// ApplyRefund submits a 7x24h or out-of-window exception refund request for a paid order.
// @ID commerceApplyRefund
// @Summary Apply for order refund
// @Description Allows full refund within 7x24h of actual course availability, or out-of-window exception manual review.
// @Tags Commerce
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body ApplyRefundRequest true "Refund application payload"
// @Success 201 {object} RefundResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /api/commerce/orders/{id}/refunds [post]
func (h *Handler) ApplyRefund(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}

	orderID := c.Param("id")
	var req ApplyRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	app, err := h.store.ApplyRefund(c.Request.Context(), principal.UserID, orderID, req.Reason, req.ExceptionType)
	if err != nil {
		switch {
		case errors.Is(err, orders.ErrOrderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrDeliveryEvidenceUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		case errors.Is(err, ErrRefundWindowExpired), errors.Is(err, ErrOrderNotEligible), errors.Is(err, ErrInvalidExceptionType):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, refundToResponse(app))
}

// ListOrderRefunds lists refund applications for a specific order owned by the user or viewed by admin.
// @ID commerceListOrderRefunds
// @Summary List refund applications for an order
// @Tags Commerce
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {array} RefundResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/commerce/orders/{id}/refunds [get]
func (h *Handler) ListOrderRefunds(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}

	orderID := c.Param("id")
	order, err := h.orderStore.GetOrder(c.Request.Context(), orderID)
	if err != nil {
		if errors.Is(err, orders.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if order.UserID != principal.UserID && !principal.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrPermissionDenied.Error()})
		return
	}

	items, err := h.store.ListRefundsByOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := make([]RefundResponse, 0, len(items))
	for _, r := range items {
		resp = append(resp, refundToResponse(r))
	}
	c.JSON(http.StatusOK, resp)
}

// GetRefund returns the status and progress of a refund application (owner or admin only).
// @ID commerceGetRefund
// @Summary Get refund application details
// @Tags Commerce
// @Produce json
// @Param id path string true "Refund ID"
// @Success 200 {object} RefundResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/commerce/refunds/{id} [get]
func (h *Handler) GetRefund(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}

	refID := c.Param("id")
	app, err := h.store.GetRefund(c.Request.Context(), refID)
	if err != nil {
		if errors.Is(err, ErrRefundNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if app.UserID != principal.UserID && !principal.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrPermissionDenied.Error()})
		return
	}

	c.JSON(http.StatusOK, refundToResponse(app))
}

// CancelRefund allows the applicant to withdraw a pending_review refund application.
// @ID commerceCancelRefund
// @Summary Withdraw pending refund application
// @Tags Commerce
// @Accept json
// @Produce json
// @Param id path string true "Refund ID"
// @Param request body CancelRefundRequest false "Optional optimistic version"
// @Success 200 {object} RefundResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/commerce/refunds/{id}/cancel [post]
func (h *Handler) CancelRefund(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}

	var req CancelRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	refID := c.Param("id")
	app, err := h.store.CancelRefund(c.Request.Context(), principal.UserID, refID, req.ExpectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, ErrRefundNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrRefundNotPending), errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, refundToResponse(app))
}

// ReviewRefund executes admin approval or rejection of a pending_review refund.
// @ID commerceReviewRefund
// @Summary Review refund application (Admin)
// @Description Approving moves refund to approved/submitting while keeping entitlement active until channel confirmation succeeds.
// @Tags Commerce
// @Accept json
// @Produce json
// @Param id path string true "Refund ID"
// @Param request body ReviewRefundRequest true "Review decision"
// @Success 200 {object} RefundResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/commerce/refunds/{id}/review [post]
func (h *Handler) ReviewRefund(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}
	if !principal.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrPermissionDenied.Error()})
		return
	}

	refID := c.Param("id")
	var req ReviewRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Decision != "approved" && req.Decision != "rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidReviewDecision.Error()})
		return
	}

	app, err := h.store.ReviewRefund(c.Request.Context(), principal.UserID, refID, req.Decision, req.Reason, req.ExpectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, ErrRefundNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrVersionConflict), errors.Is(err, ErrRefundNotPending):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInvalidReviewDecision):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, refundToResponse(app))
}

// ConfirmChannelRefund confirms the payment channel execution result of an approved refund.
// @ID commerceConfirmRefundChannel
// @Summary Confirm refund channel execution result (Channel callback or Admin)
// @Description Updates channelState (success/failed/unknown). Only on success is the order entitlement revoked and order marked refunded.
// @Tags Commerce
// @Accept json
// @Produce json
// @Param id path string true "Refund ID"
// @Param X-Service-Token header string false "Internal service token for gateway callbacks"
// @Param X-Webhook-Signature header string false "Optional HMAC-SHA256 hex signature"
// @Param request body ChannelConfirmRefundRequest true "Channel refund result"
// @Success 200 {object} RefundResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/commerce/refunds/{id}/channel-confirm [post]
func (h *Handler) ConfirmChannelRefund(c *gin.Context) {
	refID := c.Param("id")
	var req ChannelConfirmRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Authorize via X-Service-Token, HMAC-SHA256 webhook signature, OR authenticated admin.
	authorized := auth.VerifyServiceToken(c, h.serviceToken)
	if !authorized {
		sig := strings.TrimSpace(req.Signature)
		if sig == "" {
			sig = strings.TrimSpace(c.GetHeader("X-Webhook-Signature"))
		}
		if sig != "" && VerifyChannelConfirmSignature(h.webhookSecret, refID, req.ChannelRefundID, req.Status, sig) {
			authorized = true
		}
	}
	if !authorized {
		principal, ok := auth.ResolvePrincipal(c, h.resolver)
		if !ok {
			return
		}
		if !principal.IsAdmin() {
			c.JSON(http.StatusForbidden, gin.H{"error": ErrPermissionDenied.Error()})
			return
		}
	}

	status := ChannelState(strings.TrimSpace(req.Status))
	if status != ChannelSuccess && status != ChannelFailed && status != ChannelUnknown {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidChannelState.Error()})
		return
	}

	app, err := h.store.ConfirmChannelRefund(c.Request.Context(), refID, status, req.ChannelRefundID, req.FailureReason, req.ExpectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, ErrRefundNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrRefundNotApproved), errors.Is(err, ErrRefundTerminalState), errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInvalidChannelState):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/commerce/refunds [get]
func (h *Handler) ListRefunds(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}
	if !principal.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrPermissionDenied.Error()})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	items, total, err := h.store.ListRefunds(c.Request.Context(), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		ID:                   r.ID,
		OrderID:              r.OrderID,
		UserID:               r.UserID,
		Reason:               r.Reason,
		ExceptionType:        r.ExceptionType,
		IsOutOfWindow:        r.IsOutOfWindow,
		AmountMinor:          r.AmountMinor,
		ApplicationState:     string(r.ApplicationState),
		ChannelState:         string(r.ChannelState),
		ChannelRefundID:      r.ChannelRefundID,
		ChannelFailureReason: r.ChannelFailureReason,
		ReviewerID:           r.ReviewerID,
		ReviewedAt:           r.ReviewedAt,
		RejectionReason:      r.RejectionReason,
		Version:              r.Version,
		FirstAvailableAt:     r.FirstAvailableAt,
		Deadline:             r.Deadline,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
}
