package app

import (
	"github.com/StephenQiu30/ai2workspace/backend/internal/agent"
	"github.com/StephenQiu30/ai2workspace/backend/internal/commerce"
	"github.com/StephenQiu30/ai2workspace/backend/internal/community"
	"github.com/StephenQiu30/ai2workspace/backend/internal/creation"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/StephenQiu30/ai2workspace/backend/internal/learning"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Options configures the HTTP server and its modules.
type Options struct {
	Agent                 agent.Config
	PlaybackSigningSecret string
	PaymentWebhookSecret  string
	SwaggerEnabled        bool
}

// New wires all modules to the same router and database. The caller owns pool
// and registers the generated Swagger document when enabling Swagger.
func New(pool *pgxpool.Pool, accounts *identity.Module, opts Options) *gin.Engine {
	grants := commerce.NewGrants(pool)
	courses := learning.New(pool, accounts.ResolveSession, grants, opts.PlaybackSigningSecret)
	discussions := community.New(pool, accounts.ResolveSession, courses.DiscussionAccess)
	billing := commerce.New(pool, grants, accounts.ResolveSession, courses, discussions.Notify, opts.PaymentWebhookSecret)
	production := creation.New(pool, accounts.ResolveSession)
	agents := agent.New(pool, opts.Agent, accounts.ResolveSession, courses.KnowledgeAccess)

	router := newRouter(pool, opts.SwaggerEnabled)
	accounts.RegisterRoutes(router)
	production.RegisterRoutes(router)
	billing.RegisterRoutes(router)
	courses.RegisterRoutes(router)
	discussions.RegisterRoutes(router)
	agents.RegisterRoutes(router)
	return router
}
