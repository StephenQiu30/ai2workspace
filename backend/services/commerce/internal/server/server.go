// Package server assembles HTTP routes for the commerce service.
package server

import (
	"context"
	"net/http"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/commerce/api"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/entitlements"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/payments"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/refunds"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// New initializes an independent commerce router with default in-memory stores.
func New(swaggerEnabled bool) *gin.Engine {
	entStore := entitlements.NewMemoryStore()

	orderStore := orders.NewMemoryStore(
		nil,
		func(userID, courseID string) (bool, error) {
			allowed, _, _, err := entStore.CheckEntitlement(context.Background(), userID, courseID, "order_check")
			return allowed, err
		},
	)

	payStore := payments.NewMemoryStore(
		orderStore,
		func(ctx context.Context, userID, courseID, orderID string) error {
			_, err := entStore.GrantEntitlement(ctx, userID, courseID, orderID)
			return err
		},
	)

	refundStore := refunds.NewMemoryStore(orderStore, entStore, nil)

	return NewWithStores(swaggerEnabled, orderStore, payStore, entStore, refundStore)
}

// NewWithStores returns a router configured with specific stores.
func NewWithStores(
	swaggerEnabled bool,
	orderStore orders.Store,
	payStore payments.Store,
	entStore entitlements.Store,
	refundStore refunds.Store,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	orderHandler := orders.NewHandler(orderStore)
	payHandler := payments.NewHandler(payStore, orderStore)
	entHandler := entitlements.NewHandler(entStore)
	refundHandler := refunds.NewHandler(refundStore)

	// Health
	router.GET("/api/commerce/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"service": "commerce", "status": "ok"})
	})

	// Prices & Orders
	router.POST("/api/commerce/courses/:id/price", orderHandler.SetCoursePrice)
	router.GET("/api/commerce/courses/:id/price", orderHandler.GetCoursePrice)
	router.POST("/api/commerce/orders", orderHandler.CreateOrder)
	router.GET("/api/commerce/orders/:id", orderHandler.GetOrder)
	router.GET("/api/commerce/orders", orderHandler.ListOrders)

	// Payments
	router.POST("/api/commerce/orders/:id/payment-attempts", payHandler.CreatePaymentAttempt)
	router.POST("/api/commerce/payments/webhook", payHandler.ConfirmPaymentWebhook)
	router.GET("/api/commerce/orders/:id/payment-attempts", payHandler.ListPaymentAttempts)

	// Entitlements
	router.GET("/api/commerce/me/entitlements", entHandler.ListMyEntitlements)
	router.POST("/internal/commerce/entitlements/check", entHandler.CheckEntitlement)

	// Refunds
	router.POST("/api/commerce/orders/:id/refunds", refundHandler.ApplyRefund)
	router.GET("/api/commerce/refunds/:id", refundHandler.GetRefund)
	router.POST("/api/commerce/refunds/:id/review", refundHandler.ReviewRefund)
	router.GET("/api/commerce/refunds", refundHandler.ListRefunds)

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	})

	if swaggerEnabled {
		router.GET("/swagger/commerce/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	return router
}
