// Package identity owns accounts, sessions, and account access.
package identity

import (
	"context"

	httpapi "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http"

	accesshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/access"
	accountshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/accounts"
	sessionshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/sessions"
	accesspg "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/postgres/access"
	accountspg "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/postgres/accounts"
	sessionspg "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/postgres/sessions"
	accessapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/access"
	accountsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/accounts"
	sessionsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/sessions"
	sessionsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/sessions"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionCookieName is the opaque session cookie issued by identity.
const SessionCookieName = sessionsapp.SessionCookieName

var (
	// ErrInvalidSession indicates a missing or expired session.
	ErrInvalidSession = sessionsdomain.ErrInvalidSession
	// ErrInactiveAccount indicates a session whose account is inactive.
	ErrInactiveAccount = sessionsdomain.ErrInactiveAccount
)

// Actor is the account information available to other modules.
type Actor struct {
	ID            string
	Email         string
	Nickname      string
	EmailVerified bool
	Roles         []string
	Status        string
}

// SessionResolver resolves an opaque session token.
type SessionResolver func(ctx context.Context, token string) (*Actor, error)

// Module owns identity's storage and HTTP handlers.
type Module struct {
	routes httpapi.Routes
	users  accountsapp.Store
	tokens sessionsapp.Store
}

// New builds identity with PostgreSQL storage.
func New(pool *pgxpool.Pool, cfg Config) *Module {
	return NewWithStores(accountspg.NewPostgresStore(pool), sessionspg.NewPostgresStore(pool), accesspg.NewPostgresStore(pool), cfg.options())
}

// NewWithStores builds identity with explicit stores.
func NewWithStores(users accountsapp.Store, tokens sessionsapp.Store, invites accessapp.Store, opts Options) *Module {
	return &Module{
		routes: httpapi.Routes{
			Accounts: accountshttp.NewHandlerWithOptions(users, tokens, accountshttp.Options{
				DevExposeTokens: opts.DevExposeTokens,
				Mailer:          opts.Mailer,
				AuthLimiter:     opts.AuthLimiter,
				MailLimiter:     opts.MailLimiter,
			}),
			Sessions: sessionshttp.NewHandlerWithOptions(tokens, users, sessionshttp.Options{
				CookieSecure: opts.CookieSecure,
				AuthLimiter:  opts.AuthLimiter,
			}),
			Access: accesshttp.NewHandlerWithOptions(invites, users, tokens, accesshttp.Options{
				DevExposeTokens: opts.DevExposeTokens,
				Mailer:          opts.Mailer,
			}),
		},
		users:  users,
		tokens: tokens,
	}
}

// BootstrapAdmin initializes a verified administrator and returns its account ID.
func (m *Module) BootstrapAdmin(ctx context.Context, email, password string) (string, error) {
	account, err := m.users.EnsureBootstrapAdmin(ctx, email, password)
	if err != nil {
		return "", err
	}
	return account.ID, nil
}

// ResolveSession looks up the active account for a session.
func (m *Module) ResolveSession(ctx context.Context, token string) (*Actor, error) {
	account, err := sessionsapp.Resolve(ctx, m.tokens, m.users, token)
	if err != nil {
		return nil, err
	}
	return &Actor{
		ID:            account.ID,
		Email:         account.Email,
		Nickname:      account.Nickname,
		EmailVerified: account.EmailVerified,
		Roles:         account.Roles,
		Status:        account.Status,
	}, nil
}

// RegisterRoutes adds identity endpoints to the application router.
func (m *Module) RegisterRoutes(router gin.IRouter) { m.routes.RegisterRoutes(router) }
