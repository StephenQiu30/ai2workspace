package payments

import (
	"errors"
	"net/http"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
	"github.com/gin-gonic/gin"
)

// CreatePaymentAttemptRequest defines payload to start paying for an order.
type CreatePaymentAttemptRequest struct {
	Channel string `json:"channel" binding:"required"`
}

// PaymentAttemptResponse represents serialized payment attempt.
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

// WebhookConfirmationRequest represents payload received from payment gateway webhook.
type WebhookConfirmationRequest struct {
	OrderID        string `json:"orderId" binding:"required"`
	Channel        string `json:"channel" binding:"required"`
	ChannelOrderID string `json:"channelOrderId" binding:"required"`
	AmountMinor    int64  `json:"amountMinor" binding:"required"`
	Currency       string `json:"currency" binding:"required"`
	Signature      string `json:"signature" binding:"required"`
}

// Handler manages payment attempt HTTP endpoints.
type Handler struct {
	paymentStore Store
	orderStore   orders.Store
}

// NewHandler creates a payments Handler.
func NewHandler(paymentStore Store, orderStore orders.Store) *Handler {
	return &Handler{
		paymentStore: paymentStore,
		orderStore:   orderStore,
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
// @Failure 404 {object} map[string]string
// @Router /api/commerce/orders/{id}/payment-attempts [post]
func (h *Handler) CreatePaymentAttempt(c *gin.Context) {
	orderID := c.Param("id")
	var req CreatePaymentAttemptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := h.orderStore.GetOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if order.Status != orders.StatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order is not in pending state"})
		return
	}

	att, err := h.paymentStore.CreatePaymentAttempt(c.Request.Context(), orderID, req.Channel, order.AmountMinor, order.Currency)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, PaymentAttemptResponse{
		ID:             att.ID,
		OrderID:        att.OrderID,
		Channel:        att.Channel,
		ChannelOrderID: att.ChannelOrderID,
		AmountMinor:    att.AmountMinor,
		Currency:       att.Currency,
		Status:         string(att.Status),
		PaymentAction:  "mock_gateway_redirect_url",
		CreatedAt:      att.CreatedAt,
	})
}

// ConfirmPaymentWebhook processes payment callback facts from channel.
// @ID commerceHandlePaymentWebhook
// @Summary Handle payment callback confirmation
// @Description Enforces facts verification: amount, currency, and channel signature.
// @Tags Commerce
// @Accept json
// @Produce json
// @Param request body WebhookConfirmationRequest true "Payment callback fact"
// @Success 200 {object} PaymentAttemptResponse
// @Failure 400 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Router /api/commerce/payments/webhook [post]
func (h *Handler) ConfirmPaymentWebhook(c *gin.Context) {
	var req WebhookConfirmationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	att, err := h.paymentStore.ConfirmPayment(
		c.Request.Context(),
		req.OrderID,
		req.Channel,
		req.ChannelOrderID,
		req.AmountMinor,
		req.Currency,
		req.Signature,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidPaymentFacts):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, PaymentAttemptResponse{
		ID:             att.ID,
		OrderID:        att.OrderID,
		Channel:        att.Channel,
		ChannelOrderID: att.ChannelOrderID,
		AmountMinor:    att.AmountMinor,
		Currency:       att.Currency,
		Status:         string(att.Status),
		CreatedAt:      att.CreatedAt,
	})
}

// ListPaymentAttempts returns all payment attempts for an order.
// @ID commerceListPaymentAttempts
// @Summary List payment attempts for order
// @Tags Commerce
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {array} PaymentAttemptResponse
// @Router /api/commerce/orders/{id}/payment-attempts [get]
func (h *Handler) ListPaymentAttempts(c *gin.Context) {
	orderID := c.Param("id")
	attempts, err := h.paymentStore.ListAttemptsByOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := make([]PaymentAttemptResponse, 0, len(attempts))
	for _, a := range attempts {
		resp = append(resp, PaymentAttemptResponse{
			ID:             a.ID,
			OrderID:        a.OrderID,
			Channel:        a.Channel,
			ChannelOrderID: a.ChannelOrderID,
			AmountMinor:    a.AmountMinor,
			Currency:       a.Currency,
			Status:         string(a.Status),
			CreatedAt:      a.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, resp)
}
