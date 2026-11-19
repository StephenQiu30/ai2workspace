package orders

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler manages order and pricing endpoints.
type Handler struct {
	store Store
}

// NewHandler creates an orders Handler.
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// SetCoursePrice sets or updates the commercial price for a course.
// @ID commerceSetCoursePrice
// @Summary Set course purchase price
// @Tags Commerce
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param request body SetPriceRequest true "Price details"
// @Success 200 {object} PriceResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/commerce/courses/{id}/price [post]
func (h *Handler) SetCoursePrice(c *gin.Context) {
	courseID := c.Param("id")
	var req SetPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	price, err := h.store.SetCoursePrice(c.Request.Context(), courseID, req.AmountMinor, req.Currency, req.ExpectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, PriceResponse{
		CourseID:    price.CourseID,
		AmountMinor: price.AmountMinor,
		Currency:    price.Currency,
		Version:     price.Version,
		UpdatedAt:   price.UpdatedAt,
	})
}

// GetCoursePrice gets the current price of a course.
// @ID commerceGetCoursePrice
// @Summary Get course price
// @Tags Commerce
// @Produce json
// @Param id path string true "Course ID"
// @Success 200 {object} PriceResponse
// @Failure 404 {object} map[string]string
// @Router /api/commerce/courses/{id}/price [get]
func (h *Handler) GetCoursePrice(c *gin.Context) {
	courseID := c.Param("id")
	price, err := h.store.GetCoursePrice(c.Request.Context(), courseID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, PriceResponse{
		CourseID:    price.CourseID,
		AmountMinor: price.AmountMinor,
		Currency:    price.Currency,
		Version:     price.Version,
		UpdatedAt:   price.UpdatedAt,
	})
}

// CreateOrder initiates a course purchase transaction.
// @ID commerceCreateOrder
// @Summary Create course order
// @Tags Commerce
// @Accept json
// @Produce json
// @Param request body CreateOrderRequest true "Order creation payload"
// @Success 201 {object} OrderResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/commerce/orders [post]
func (h *Handler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "mock_buyer"
	}

	order, err := h.store.CreateOrder(c.Request.Context(), userID, req.CourseID, req.IdempotencyKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrAlreadyEntitled), errors.Is(err, ErrIdempotencyMismatch):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, orderToResponse(order))
}

// GetOrder fetches order transaction details.
// @ID commerceGetOrder
// @Summary Get order details
// @Tags Commerce
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} OrderResponse
// @Failure 404 {object} map[string]string
// @Router /api/commerce/orders/{id} [get]
func (h *Handler) GetOrder(c *gin.Context) {
	orderID := c.Param("id")
	order, err := h.store.GetOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orderToResponse(order))
}

// ListOrders lists orders belonging to the authenticated user.
// @ID commerceListOrders
// @Summary List current user orders
// @Tags Commerce
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} OrderListResponse
// @Router /api/commerce/orders [get]
func (h *Handler) ListOrders(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "mock_buyer"
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	items, total, err := h.store.ListOrders(c.Request.Context(), userID, page, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	respList := make([]OrderResponse, 0, len(items))
	for _, o := range items {
		respList = append(respList, orderToResponse(o))
	}

	c.JSON(http.StatusOK, OrderListResponse{
		Items: respList,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

func orderToResponse(o *Order) OrderResponse {
	return OrderResponse{
		ID:             o.ID,
		UserID:         o.UserID,
		CourseID:       o.CourseID,
		AmountMinor:    o.AmountMinor,
		Currency:       o.Currency,
		Status:         string(o.Status),
		IdempotencyKey: o.IdempotencyKey,
		PolicyVersion:  o.PolicyVersion,
		PaidAt:         o.PaidAt,
		CreatedAt:      o.CreatedAt,
		UpdatedAt:      o.UpdatedAt,
	}
}
