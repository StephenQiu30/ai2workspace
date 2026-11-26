// Package http binds endpoints for the commerce module.
package http

import (
	entitlementshttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/entitlements"
	ordershttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/orders"
	paymentshttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/payments"
	refundshttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/refunds"
	"github.com/gin-gonic/gin"
)

// Routes holds the HTTP handlers for the commerce module.
type Routes struct {
	Orders       *ordershttp.Handler
	Payments     *paymentshttp.Handler
	Entitlements *entitlementshttp.Handler
	Refunds      *refundshttp.Handler
}

// RegisterRoutes adds commerce endpoints to the application router.
func (r Routes) RegisterRoutes(router gin.IRouter) {
	// Prices & Orders
	router.POST("/api/commerce/courses/:id/price", r.Orders.SetCoursePrice)
	router.GET("/api/commerce/courses/:id/price", r.Orders.GetCoursePrice)
	router.POST("/api/commerce/orders", r.Orders.CreateOrder)
	router.GET("/api/commerce/orders", r.Orders.ListOrders)
	router.GET("/api/commerce/orders/:id", r.Orders.GetOrder)
	router.POST("/api/commerce/orders/:id/cancel", r.Orders.CancelOrder)

	// Payments
	router.POST("/api/commerce/orders/:id/payment-attempts", r.Payments.CreatePaymentAttempt)
	router.GET("/api/commerce/orders/:id/payment-attempts", r.Payments.ListPaymentAttempts)
	router.GET("/api/commerce/orders/:id/payment-status", r.Payments.GetPaymentStatus)
	router.POST("/api/commerce/payments/webhook", r.Payments.ConfirmPaymentWebhook)

	// Entitlements
	router.GET("/api/commerce/me/entitlements", r.Entitlements.ListMyEntitlements)

	// Refunds
	router.POST("/api/commerce/orders/:id/refunds", r.Refunds.ApplyRefund)
	router.GET("/api/commerce/orders/:id/refunds", r.Refunds.ListOrderRefunds)
	router.GET("/api/commerce/refunds", r.Refunds.ListRefunds)
	router.GET("/api/commerce/refunds/:id", r.Refunds.GetRefund)
	router.POST("/api/commerce/refunds/:id/cancel", r.Refunds.CancelRefund)
	router.POST("/api/commerce/refunds/:id/review", r.Refunds.ReviewRefund)
	router.POST("/api/commerce/refunds/:id/channel-confirm", r.Refunds.ConfirmChannelRefund)
}
