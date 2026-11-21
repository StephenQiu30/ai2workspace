package payments

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
	"github.com/gin-gonic/gin"
)

// CreatePaymentAttemptRequest defines payload to start paying for an order.
type CreatePaymentAttemptRequest struct {
	Channel        string `json:"channel" binding:"required"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

// PaymentAttemptResponse represents a serialized payment attempt.
type PaymentAttemptResponse struct {
	ID             string    `json:"id"`
	OrderID        string    `json:"orderId"`
	Channel        string    `json:"channel"`
	ChannelOrderID string    `json:"channelOrderId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	Status         string    `json:"status"`
	PaymentAction  string    `json:"paymentAction,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// PaymentStatusResponse represents reconciled order and payment status.
type PaymentStatusResponse struct {
	OrderID       string                  `json:"orderId"`
	OrderStatus   string                  `json:"orderStatus"`
	PaymentStatus string                  `json:"paymentStatus"`
	LatestAttempt *PaymentAttemptResponse `json:"latestAttempt,omitempty"`
	Recovered     bool                    `json:"recovered"`
	Message       string                  `json:"message,omitempty"`
	CheckedAt     time.Time               `json:"checkedAt"`
}

// WebhookConfirmationRequest represents payload received from payment gateway webhook.
type WebhookConfirmationRequest struct {
	OrderID        string `json:"orderId" binding:"required"`
	Channel        string `json:"channel" binding:"required"`
	ChannelOrderID string `json:"channelOrderId" binding:"required"`
	AmountMinor    int64  `json:"amountMinor" binding:"required"`
	Currency       string `json:"currency" binding:"required"`
	Signature      string `json:"signature"`
}

// Handler manages payment attempt HTTP endpoints.
type Handler struct {
	paymentStore Store
	orderStore   orders.Store
	resolver     auth.Resolver
}

// NewHandler creates a payments Handler.
func NewHandler(paymentStore Store, orderStore orders.Store, resolver auth.Resolver) *Handler {
	return &Handler{
		paymentStore: paymentStore,
		orderStore:   orderStore,
		resolver:     resolver,
	}
}

// CreatePaymentAttempt initiates a payment attempt on an existing order.
// @ID commerceCreatePaymentAttempt
// @Summary Create payment attempt
// @Tags Commerce
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body CreatePaymentAttemptRequest true "Payment attempt payload"
// @Success 201 {object} PaymentAttemptResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/commerce/orders/{id}/payment-attempts [post]
func (h *Handler) CreatePaymentAttempt(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}

	orderID := c.Param("id")
	var req CreatePaymentAttemptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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
		c.JSON(http.StatusForbidden, gin.H{"error": auth.ErrForbidden.Error()})
		return
	}

	if order.Status != orders.StatusPendingPayment {
		c.JSON(http.StatusConflict, gin.H{"error": ErrOrderNotPending.Error()})
		return
	}

	att, err := h.paymentStore.CreatePaymentAttempt(c.Request.Context(), orderID, req.Channel, order.AmountMinor, order.Currency)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := attemptToResponse(att)
	resp.PaymentAction = "mock_gateway_redirect_url"
	c.JSON(http.StatusCreated, resp)
}

// ConfirmPaymentWebhook processes payment callback facts from a channel with HMAC-SHA256 verification.
// @ID commerceHandlePaymentWebhook
// @Summary Handle payment callback confirmation
// @Description Enforces HMAC-SHA256 signature verification, amount/currency matching, and out-of-order conflict rejection.
// @Tags Commerce
// @Accept json
// @Produce json
// @Param X-Webhook-Signature header string false "Optional HMAC-SHA256 hex signature"
// @Param request body WebhookConfirmationRequest true "Payment callback fact"
// @Success 200 {object} PaymentAttemptResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Router /api/commerce/payments/webhook [post]
func (h *Handler) ConfirmPaymentWebhook(c *gin.Context) {
	var req WebhookConfirmationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sig := strings.TrimSpace(req.Signature)
	if sig == "" {
		sig = strings.TrimSpace(c.GetHeader("X-Webhook-Signature"))
	}

	att, err := h.paymentStore.ConfirmPayment(
		c.Request.Context(),
		req.OrderID,
		req.Channel,
		req.ChannelOrderID,
		req.AmountMinor,
		req.Currency,
		sig,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidPaymentFacts):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, orders.ErrOrderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrOrderStateConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, attemptToResponse(att))
}

// ListPaymentAttempts returns all payment attempts for an order.
// @ID commerceListPaymentAttempts
// @Summary List payment attempts for order
// @Tags Commerce
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {array} PaymentAttemptResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/commerce/orders/{id}/payment-attempts [get]
func (h *Handler) ListPaymentAttempts(c *gin.Context) {
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
		c.JSON(http.StatusForbidden, gin.H{"error": auth.ErrForbidden.Error()})
		return
	}

	attempts, err := h.paymentStore.ListAttemptsByOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := make([]PaymentAttemptResponse, 0, len(attempts))
	for _, a := range attempts {
		resp = append(resp, attemptToResponse(a))
	}

	c.JSON(http.StatusOK, resp)
}

// GetPaymentStatus actively queries payment status and reconciles unknown attempts.
// @ID commerceGetPaymentStatus
// @Summary Query and recover order payment status
// @Tags Commerce
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} PaymentStatusResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/commerce/orders/{id}/payment-status [get]
func (h *Handler) GetPaymentStatus(c *gin.Context) {
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
		c.JSON(http.StatusForbidden, gin.H{"error": auth.ErrForbidden.Error()})
		return
	}

	report, err := h.paymentStore.QueryPaymentStatus(c.Request.Context(), orderID)
	if err != nil {
		switch {
		case errors.Is(err, orders.ErrOrderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrOrderStateConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInvalidPaymentFacts):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	var latestResp *PaymentAttemptResponse
	if report.LatestAttempt != nil {
		r := attemptToResponse(report.LatestAttempt)
		latestResp = &r
	}

	c.JSON(http.StatusOK, PaymentStatusResponse{
		OrderID:       report.OrderID,
		OrderStatus:   report.OrderStatus,
		PaymentStatus: report.PaymentStatus,
		LatestAttempt: latestResp,
		Recovered:     report.Recovered,
		Message:       report.Message,
		CheckedAt:     report.CheckedAt,
	})
}

func attemptToResponse(a *PaymentAttempt) PaymentAttemptResponse {
	return PaymentAttemptResponse{
		ID:             a.ID,
		OrderID:        a.OrderID,
		Channel:        a.Channel,
		ChannelOrderID: a.ChannelOrderID,
		AmountMinor:    a.AmountMinor,
		Currency:       a.Currency,
		Status:         string(a.Status),
		CreatedAt:      a.CreatedAt,
	}
}
