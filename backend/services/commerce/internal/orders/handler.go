package orders

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/auth"
	"github.com/gin-gonic/gin"
)

// Handler manages order and pricing endpoints.
type Handler struct {
	store    Store
	resolver auth.Resolver
}

// NewHandler creates an orders Handler.
func NewHandler(store Store, resolver auth.Resolver) *Handler {
	return &Handler{
		store:    store,
		resolver: resolver,
	}
}

// SetCoursePrice sets or updates the commercial price for a course.
// @ID commerceSetCoursePrice
// @Summary Set course purchase price (Admin)
// @Tags Commerce
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param request body SetPriceRequest true "Price details"
// @Success 200 {object} PriceResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/commerce/courses/{id}/price [post]
func (h *Handler) SetCoursePrice(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}
	if !principal.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": auth.ErrForbidden.Error()})
		return
	}

	courseID := c.Param("id")
	var req SetPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	price, err := h.store.SetCoursePrice(c.Request.Context(), courseID, *req.AmountMinor, req.Currency, req.ExpectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInvalidAmount), errors.Is(err, ErrUnsupportedCurrency):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		if errors.Is(err, ErrPriceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /api/commerce/orders [post]
func (h *Handler) CreateOrder(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}
	if !principal.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": auth.ErrEmailNotVerified.Error()})
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := h.store.CreateOrder(c.Request.Context(), principal.UserID, req.CourseID, req.IdempotencyKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrAlreadyEntitled), errors.Is(err, ErrIdempotencyMismatch), errors.Is(err, ErrCourseNotForSale):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCourseNotFound), errors.Is(err, ErrPriceNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrFreeCourseOrderNotAllowed):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUpstreamUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/commerce/orders/{id} [get]
func (h *Handler) GetOrder(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}

	orderID := c.Param("id")
	order, err := h.store.GetOrder(c.Request.Context(), orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
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
// @Failure 401 {object} map[string]string
// @Router /api/commerce/orders [get]
func (h *Handler) ListOrders(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
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

	items, total, err := h.store.ListOrders(c.Request.Context(), principal.UserID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

// CancelOrder cancels a pending_payment order owned by the authenticated user.
// @ID commerceCancelOrder
// @Summary Cancel pending payment order
// @Tags Commerce
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} OrderResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/commerce/orders/{id}/cancel [post]
func (h *Handler) CancelOrder(c *gin.Context) {
	principal, ok := auth.ResolvePrincipal(c, h.resolver)
	if !ok {
		return
	}

	orderID := c.Param("id")
	order, err := h.store.CancelOrder(c.Request.Context(), principal.UserID, orderID)
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrOrderNotPending):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, orderToResponse(order))
}

func orderToResponse(o *Order) OrderResponse {
	return OrderResponse{
		ID:             o.ID,
		UserID:         o.UserID,
		CourseID:       o.CourseID,
		CourseTitle:    o.CourseTitle,
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
