// Package commerce owns orders, payments, refunds and purchase entitlements.
package commerce

import (
	"context"
	"errors"
	"time"

	httpapi "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http"

	authhttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/auth"
	entitlementshttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/entitlements"
	ordershttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/orders"
	paymentshttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/payments"
	refundshttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/refunds"
	orderspg "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/postgres/orders"
	paymentspg "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/postgres/payments"
	refundspg "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/postgres/refunds"
	entitlementsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/entitlements"
	ordersapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/orders"
	paymentsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/payments"
	refundsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/refunds"
	ordersdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/orders"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/StephenQiu30/ai2workspace/backend/internal/learning"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps bundles dependencies for constructing the commerce router (used in production and integration tests).
type Deps struct {
	WebhookSecret    string
	AuthResolver     authhttp.Resolver
	OrderStore       ordersapp.Store
	PaymentStore     paymentsapp.Store
	EntitlementStore entitlementsapp.Store
	RefundStore      refundsapp.Store
}

// Module owns commerce dependencies and HTTP handlers.
type Module struct {
	routes httpapi.Routes
}

// CourseCatalog supplies the learning facts needed for orders and refunds.
type CourseCatalog interface {
	SaleSnapshot(ctx context.Context, courseID string) (*learning.SaleSnapshot, error)
	DeliveryEvidence(ctx context.Context, courseID string, notBefore time.Time) (*time.Time, error)
}

// New builds commerce with PostgreSQL storage and its application dependencies.
func New(pool *pgxpool.Pool, grants *Grants, resolve identity.SessionResolver, catalog CourseCatalog, notify func(context.Context, string, string, string, string, string) error, webhookSecret string) *Module {
	orderStore := orderspg.NewPostgresStore(pool, func(ctx context.Context, courseID string) (*ordersdomain.SaleSnapshot, error) {
		snapshot, err := catalog.SaleSnapshot(ctx, courseID)
		if err != nil {
			return nil, err
		}
		return &ordersdomain.SaleSnapshot{
			CourseID:         snapshot.CourseID,
			Title:            snapshot.Title,
			IsFree:           snapshot.IsFree,
			SaleState:        snapshot.SaleState,
			PublishedVersion: snapshot.PublishedVersion,
			Deliverable:      snapshot.Deliverable,
		}, nil
	}, func(ctx context.Context, userID, courseID string) (bool, error) {
		allowed, _, err := grants.CheckEntitlement(ctx, userID, courseID, "order_check")
		return allowed, err
	})

	return NewWithDeps(Deps{
		WebhookSecret:    webhookSecret,
		AuthResolver:     sessionAuthenticator(resolve),
		EntitlementStore: grants.store,
		OrderStore:       orderStore,
		PaymentStore: paymentspg.NewPostgresStore(pool, orderStore, func(ctx context.Context, userID, courseID, orderID string) error {
			_, err := grants.store.GrantEntitlement(ctx, userID, courseID, orderID)
			return err
		}, paymentsapp.StoreOptions{WebhookSecret: webhookSecret, Notifier: notify}),
		RefundStore: refundspg.NewPostgresStore(pool, orderStore, grants.store, catalog.DeliveryEvidence, notify),
	})
}

func sessionAuthenticator(resolve identity.SessionResolver) authhttp.Resolver {
	return func(c *gin.Context) (*authhttp.Principal, error) {
		token, err := c.Cookie(identity.SessionCookieName)
		if err != nil {
			return nil, authhttp.ErrUnauthenticated
		}
		actor, err := resolve(c.Request.Context(), token)
		switch {
		case errors.Is(err, identity.ErrInvalidSession):
			return nil, authhttp.ErrUnauthenticated
		case errors.Is(err, identity.ErrInactiveAccount):
			return nil, authhttp.ErrAccountSuspended
		case err != nil:
			return nil, authhttp.ErrIdentityUnavailable
		}
		return &authhttp.Principal{
			UserID:        actor.ID,
			Email:         actor.Email,
			EmailVerified: actor.EmailVerified,
			Roles:         actor.Roles,
			AccountStatus: actor.Status,
		}, nil
	}
}

// NewWithDeps builds commerce with explicit dependencies.
func NewWithDeps(deps Deps) *Module {
	resolver := deps.AuthResolver
	if resolver == nil {
		resolver = authhttp.DenyAllResolver
	}

	orderHandler := ordershttp.NewHandler(deps.OrderStore, resolver)
	payHandler := paymentshttp.NewHandler(deps.PaymentStore, deps.OrderStore, resolver)
	entHandler := entitlementshttp.NewHandler(deps.EntitlementStore, resolver)
	refundHandler := refundshttp.NewHandler(deps.RefundStore, deps.OrderStore, resolver, deps.WebhookSecret)

	return &Module{
		routes: httpapi.Routes{
			Orders:       orderHandler,
			Payments:     payHandler,
			Entitlements: entHandler,
			Refunds:      refundHandler,
		},
	}
}

// RegisterRoutes adds commerce endpoints to the application router.
func (m *Module) RegisterRoutes(router gin.IRouter) { m.routes.RegisterRoutes(router) }
