package agent

import (
	"context"
	"errors"

	"github.com/StephenQiu30/ai2workspace/backend/internal/agent/adapter/http/assistant"
	agentpg "github.com/StephenQiu30/ai2workspace/backend/internal/agent/adapter/postgres/agent"
	knowledgepg "github.com/StephenQiu30/ai2workspace/backend/internal/agent/adapter/postgres/knowledge"
	usagepg "github.com/StephenQiu30/ai2workspace/backend/internal/agent/adapter/postgres/usage"
	providerapi "github.com/StephenQiu30/ai2workspace/backend/internal/agent/adapter/provider"
	agentapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/agent"
	knowledgeapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/knowledge"
	providerapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/provider"
	usageapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/usage"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps allows tests and callers to inject explicit collaborators without trusting unverified headers.
type Deps struct {
	DBPool              *pgxpool.Pool
	SessionResolver     assistant.SessionResolver
	CourseAccessChecker knowledgeapp.UserAccessChecker
	UsageService        *usageapp.Service
	KnowledgeService    *knowledgeapp.Service
	ModelClient         *providerapi.Client
	SearchEngine        providerapp.SearchEngine
	AgentService        *agentapp.Service
}

// Module wires the Agent use cases and adapters.
type Module struct {
	http *assistant.Handler
}

// RegisterRoutes adds Agent endpoints to the application router.
func (m *Module) RegisterRoutes(router gin.IRouter) { m.http.RegisterRoutes(router) }

// New builds assistant with PostgreSQL storage and its application dependencies.
func New(pool *pgxpool.Pool, cfg Config, resolve identity.SessionResolver, checkCourse knowledgeapp.UserAccessChecker) *Module {
	return NewWithDeps(cfg, Deps{
		DBPool:              pool,
		CourseAccessChecker: checkCourse,
		SessionResolver: func(ctx context.Context, token string) (*assistant.Actor, error) {
			actor, err := resolve(ctx, token)
			if errors.Is(err, identity.ErrInactiveAccount) {
				return nil, assistant.ErrAccountSuspended
			}
			if err != nil {
				return nil, assistant.ErrUnauthorized
			}
			return &assistant.Actor{
				UserID:        actor.ID,
				Email:         actor.Email,
				EmailVerified: actor.EmailVerified,
				Roles:         actor.Roles,
				AccountStatus: actor.Status,
			}, nil
		},
	})
}

// NewWithDeps builds assistant with explicit dependencies.
func NewWithDeps(cfg Config, deps Deps) *Module {
	pool := deps.DBPool
	resolver := deps.SessionResolver
	if resolver == nil {
		resolver = func(context.Context, string) (*assistant.Actor, error) { return nil, assistant.ErrUnauthorized }
	}
	accessChecker := deps.CourseAccessChecker

	usageSvc := deps.UsageService
	if usageSvc == nil {
		var store usageapp.Store
		if pool != nil {
			store = usagepg.NewStore(pool)
		}
		usageSvc = usageapp.NewServiceWithStore(store)
	}

	knowledgeSvc := deps.KnowledgeService
	if knowledgeSvc == nil {
		var store knowledgeapp.Store
		if pool != nil {
			store = knowledgepg.NewStore(pool)
		}
		knowledgeSvc = knowledgeapp.NewServiceWithStore(accessChecker, usageSvc, store)
	}

	modelCli := deps.ModelClient
	if modelCli == nil {
		modelCli = providerapi.NewClientWithOptions(providerapi.ClientOptions{
			Endpoints: map[string]providerapi.Endpoint{
				"deepseek":   cfg.DeepSeek,
				"openrouter": cfg.OpenRouter,
				"volcengine": cfg.Volcengine,
				"openai":     cfg.OpenAI,
			},
			AllowLocalSimulation: cfg.AllowLocalSimulation,
			DBPool:               pool,
		})
	}

	searchEng := deps.SearchEngine
	if searchEng == nil {
		searchEng = providerapi.NewSearchEngine(&cfg)
	}

	agentSvc := deps.AgentService
	if agentSvc == nil {
		var store agentapp.Store
		if pool != nil {
			store = agentpg.NewStore(pool)
		}
		agentSvc = agentapp.NewServiceWithStore(usageSvc, knowledgeSvc, modelCli, searchEng, store)
	}

	return &Module{http: assistant.NewHandler(resolver, usageSvc, knowledgeSvc, modelCli, agentSvc)}
}
