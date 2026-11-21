// Package server assembles HTTP routes and dependencies for the commerce service.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/commerce/api"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/entitlements"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/health"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/payments"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/refunds"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Deps bundles dependencies for constructing the commerce router (used in production and integration tests).
type Deps struct {
	SwaggerEnabled   bool
	ServiceToken     string
	WebhookSecret    string
	AuthResolver     auth.Resolver
	OrderStore       orders.Store
	PaymentStore     payments.Store
	EntitlementStore entitlements.Store
	RefundStore      refunds.Store
}

// New initializes a commerce router with in-memory stores and default fail-closed auth.
func New(swaggerEnabled bool) *gin.Engine {
	cfg := &config.Config{
		SwaggerEnabled:       swaggerEnabled,
		ServiceToken:         "identity-service-secret",
		PaymentWebhookSecret: "dev-commerce-webhook-secret",
		HTTPTimeout:          30 * time.Second,
	}
	router, _, _ := NewFromConfig(context.Background(), cfg)
	return router
}

// NewFromConfig builds the commerce router from runtime configuration, connecting PostgreSQL when DATABASE_URL is set.
func NewFromConfig(ctx context.Context, cfg *config.Config) (*gin.Engine, func(), error) {
	if cfg == nil {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return nil, nil, err
		}
	}

	clientTimeout := cfg.HTTPTimeout
	if clientTimeout <= 0 || clientTimeout > 10*time.Second {
		clientTimeout = 10 * time.Second
	}
	httpClient := &http.Client{Timeout: clientTimeout}

	authResolver := auth.NewIdentityResolver(cfg.IdentityServiceURL, cfg.ServiceToken, httpClient)
	saleVerifier := NewLearningSaleVerifier(cfg.LearningServiceURL, cfg.ServiceToken, httpClient)
	evidenceProvider := NewLearningDeliveryEvidenceProvider(cfg.LearningServiceURL, cfg.ServiceToken, httpClient)
	notifier := NewCommunityNotifier(cfg.CommunityServiceURL, cfg.ServiceToken, httpClient)

	var (
		entStore    entitlements.Store
		orderStore  orders.Store
		payStore    payments.Store
		refundStore refunds.Store
		cleanup     = func() {}
	)

	if cfg.DatabaseURL != "" {
		pool, err := OpenDB(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		cleanup = pool.Close
		slog.Info("commerce service connected to PostgreSQL")

		pgEnt := entitlements.NewPostgresStore(pool)
		pgOrders := orders.NewPostgresStore(pool, saleVerifier, func(c context.Context, userID, courseID string) (bool, error) {
			allowed, _, _, err := pgEnt.CheckEntitlement(c, userID, courseID, "order_check")
			return allowed, err
		})
		pgPayments := payments.NewPostgresStore(pool, pgOrders, func(c context.Context, userID, courseID, orderID string) error {
			_, err := pgEnt.GrantEntitlement(c, userID, courseID, orderID)
			return err
		}, payments.StoreOptions{
			WebhookSecret: cfg.PaymentWebhookSecret,
			Notifier:      notifier,
		})
		pgRefunds := refunds.NewPostgresStore(pool, pgOrders, pgEnt, evidenceProvider, notifier)

		entStore = pgEnt
		orderStore = pgOrders
		payStore = pgPayments
		refundStore = pgRefunds
	} else {
		memEnt := entitlements.NewMemoryStore()
		memOrders := orders.NewMemoryStore(saleVerifier, func(c context.Context, userID, courseID string) (bool, error) {
			allowed, _, _, err := memEnt.CheckEntitlement(c, userID, courseID, "order_check")
			return allowed, err
		})
		memPayments := payments.NewMemoryStore(memOrders, func(c context.Context, userID, courseID, orderID string) error {
			_, err := memEnt.GrantEntitlement(c, userID, courseID, orderID)
			return err
		}, payments.StoreOptions{
			WebhookSecret: cfg.PaymentWebhookSecret,
			Notifier:      notifier,
		})
		memRefunds := refunds.NewMemoryStore(memOrders, memEnt, evidenceProvider, notifier)

		entStore = memEnt
		orderStore = memOrders
		payStore = memPayments
		refundStore = memRefunds
	}

	router := NewWithDeps(Deps{
		SwaggerEnabled:   cfg.SwaggerEnabled,
		ServiceToken:     cfg.ServiceToken,
		WebhookSecret:    cfg.PaymentWebhookSecret,
		AuthResolver:     authResolver,
		OrderStore:       orderStore,
		PaymentStore:     payStore,
		EntitlementStore: entStore,
		RefundStore:      refundStore,
	})
	return router, cleanup, nil
}

// NewWithDeps returns a router wired with explicitly injected dependencies (and fail-closed auth if resolver is nil).
func NewWithDeps(deps Deps) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	resolver := deps.AuthResolver
	if resolver == nil {
		resolver = auth.DenyAllResolver
	}

	orderHandler := orders.NewHandler(deps.OrderStore, resolver)
	payHandler := payments.NewHandler(deps.PaymentStore, deps.OrderStore, resolver)
	entHandler := entitlements.NewHandler(deps.EntitlementStore, resolver, deps.ServiceToken)
	refundHandler := refunds.NewHandler(deps.RefundStore, deps.OrderStore, resolver, deps.ServiceToken, deps.WebhookSecret)

	// Health
	router.GET("/api/commerce/health", health.Get)

	// Prices & Orders
	router.POST("/api/commerce/courses/:id/price", orderHandler.SetCoursePrice)
	router.GET("/api/commerce/courses/:id/price", orderHandler.GetCoursePrice)
	router.POST("/api/commerce/orders", orderHandler.CreateOrder)
	router.GET("/api/commerce/orders", orderHandler.ListOrders)
	router.GET("/api/commerce/orders/:id", orderHandler.GetOrder)
	router.POST("/api/commerce/orders/:id/cancel", orderHandler.CancelOrder)

	// Payments
	router.POST("/api/commerce/orders/:id/payment-attempts", payHandler.CreatePaymentAttempt)
	router.GET("/api/commerce/orders/:id/payment-attempts", payHandler.ListPaymentAttempts)
	router.GET("/api/commerce/orders/:id/payment-status", payHandler.GetPaymentStatus)
	router.POST("/api/commerce/payments/webhook", payHandler.ConfirmPaymentWebhook)

	// Entitlements
	router.GET("/api/commerce/me/entitlements", entHandler.ListMyEntitlements)
	router.POST("/internal/commerce/entitlements/check", entHandler.CheckEntitlement)
	router.GET("/internal/commerce/users/:userId/entitlements", entHandler.ListUserEntitlementsInternal)

	// Refunds
	router.POST("/api/commerce/orders/:id/refunds", refundHandler.ApplyRefund)
	router.GET("/api/commerce/orders/:id/refunds", refundHandler.ListOrderRefunds)
	router.GET("/api/commerce/refunds", refundHandler.ListRefunds)
	router.GET("/api/commerce/refunds/:id", refundHandler.GetRefund)
	router.POST("/api/commerce/refunds/:id/cancel", refundHandler.CancelRefund)
	router.POST("/api/commerce/refunds/:id/review", refundHandler.ReviewRefund)
	router.POST("/api/commerce/refunds/:id/channel-confirm", refundHandler.ConfirmChannelRefund)

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	})

	if deps.SwaggerEnabled {
		router.GET("/swagger/commerce/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	return router
}
