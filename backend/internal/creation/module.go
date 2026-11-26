// Package creation assembles the original IP production module.
package creation

import (
	"github.com/StephenQiu30/ai2workspace/backend/internal/creation/adapter/http/studio"
	"github.com/StephenQiu30/ai2workspace/backend/internal/creation/adapter/postgres"
	"github.com/StephenQiu30/ai2workspace/backend/internal/creation/application"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Module wires the creation use cases and adapters.
type Module struct {
	http *studio.Handler
}

// New builds the creation module on the shared PostgreSQL pool.
func New(pool *pgxpool.Pool, resolve identity.SessionResolver) *Module {
	return &Module{http: studio.NewHandler(application.NewAccess(resolve, postgres.NewMemberships(pool)))}
}

// RegisterRoutes adds creation endpoints to the application router.
func (m *Module) RegisterRoutes(router gin.IRouter) { m.http.RegisterRoutes(router) }
