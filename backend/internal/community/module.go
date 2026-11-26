// Package community owns discussions, interactions, moderation and notifications.
package community

import (
	"context"
	"errors"

	httpapi "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http"

	authhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/auth"
	interactionhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/interaction"
	moderationhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/moderation"
	notificationhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/notification"
	postshttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/posts"
	interactionpg "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/postgres/interaction"
	moderationpg "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/postgres/moderation"
	notificationpg "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/postgres/notification"
	postspg "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/postgres/posts"
	interactionapp "github.com/StephenQiu30/ai2workspace/backend/internal/community/application/interaction"
	moderationapp "github.com/StephenQiu30/ai2workspace/backend/internal/community/application/moderation"
	notificationapp "github.com/StephenQiu30/ai2workspace/backend/internal/community/application/notification"
	postsapp "github.com/StephenQiu30/ai2workspace/backend/internal/community/application/posts"
	postsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/posts"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps encapsulates injectable stores, authentication resolvers, and runtime flags.
type Deps struct {
	PostStore           postsapp.Store
	InteractionStore    interactionapp.Store
	ModerationStore     moderationapp.Store
	NotificationStore   notificationapp.Store
	SessionResolver     authhttp.SessionResolver
	CourseAccessChecker authhttp.CourseAccessChecker
}

// Module owns community dependencies and HTTP handlers.
type Module struct {
	routes        httpapi.Routes
	notifications notificationapp.Store
}

// New builds community with PostgreSQL storage and its application dependencies.
func New(pool *pgxpool.Pool, resolve identity.SessionResolver, checkCourse authhttp.CourseAccessChecker) *Module {
	return NewWithDeps(Deps{
		PostStore:           postspg.NewPostgresStore(pool),
		InteractionStore:    interactionpg.NewPostgresStore(pool),
		ModerationStore:     moderationpg.NewPostgresStore(pool),
		NotificationStore:   notificationpg.NewPostgresStore(pool),
		CourseAccessChecker: checkCourse,
		SessionResolver: func(ctx context.Context, token string) (*authhttp.Actor, error) {
			actor, err := resolve(ctx, token)
			switch {
			case errors.Is(err, identity.ErrInvalidSession):
				return nil, authhttp.ErrUnauthenticated
			case errors.Is(err, identity.ErrInactiveAccount):
				return nil, authhttp.ErrAccountSuspended
			case err != nil:
				return nil, authhttp.ErrDependencyUnavailable
			}
			return &authhttp.Actor{
				UserID:        actor.ID,
				Email:         actor.Email,
				Nickname:      actor.Nickname,
				EmailVerified: actor.EmailVerified,
				Roles:         actor.Roles,
				AccountStatus: actor.Status,
			}, nil
		},
	})
}

// Notify records an event from another module in the recipient's notification feed.
func (m *Module) Notify(ctx context.Context, eventID, recipientID, kind, title, summary string) error {
	_, err := m.notifications.AppendNotification(ctx, eventID, recipientID, kind, title, summary, "", "")
	return err
}

// NewWithDeps builds community with explicit dependencies.
func NewWithDeps(deps Deps) *Module {
	if deps.PostStore == nil {
		deps.PostStore = postsapp.NewMemoryStore()
	}
	if deps.InteractionStore == nil {
		deps.InteractionStore = interactionapp.NewMemoryStore()
	}
	if deps.ModerationStore == nil {
		deps.ModerationStore = moderationapp.NewMemoryStore()
	}
	if deps.NotificationStore == nil {
		deps.NotificationStore = notificationapp.NewMemoryStore()
	}

	replyNotifier := func(ctx context.Context, eventID, recipientID, notifType, title, summary, targetType, targetID string) error {
		_, err := deps.NotificationStore.AppendNotification(ctx, eventID, recipientID, notifType, title, summary, targetType, targetID)
		return err
	}

	notifTargetChecker := func(ctx context.Context, userID, targetType, targetID string) bool {
		switch targetType {
		case "post":
			p, err := deps.PostStore.GetPost(ctx, targetID)
			if err != nil || p.Status != postsdomain.StatusPublished {
				return false
			}
			if p.Scope == postsdomain.ScopeCourse {
				if deps.CourseAccessChecker == nil {
					return false
				}
				allowed, err := deps.CourseAccessChecker(ctx, p.CourseID, userID)
				return err == nil && allowed
			}
			return true
		case "comment":
			cmt, err := deps.PostStore.GetComment(ctx, targetID)
			if err != nil || cmt.Status != postsdomain.StatusPublished {
				return false
			}
			p, err := deps.PostStore.GetPost(ctx, cmt.PostID)
			if err != nil || p.Status != postsdomain.StatusPublished {
				return false
			}
			if p.Scope == postsdomain.ScopeCourse {
				if deps.CourseAccessChecker == nil {
					return false
				}
				allowed, err := deps.CourseAccessChecker(ctx, p.CourseID, userID)
				return err == nil && allowed
			}
			return true
		default:
			return true
		}
	}

	postsHandler := postshttp.NewHandler(deps.PostStore, deps.SessionResolver, deps.CourseAccessChecker, replyNotifier)
	interactHandler := interactionhttp.NewHandler(deps.InteractionStore, deps.PostStore, deps.SessionResolver, deps.CourseAccessChecker)
	modHandler := moderationhttp.NewHandler(deps.ModerationStore, deps.PostStore, deps.NotificationStore, deps.SessionResolver, deps.CourseAccessChecker)
	notifHandler := notificationhttp.NewHandler(deps.NotificationStore, deps.SessionResolver, notifTargetChecker)

	return &Module{
		routes: httpapi.Routes{
			Posts:        postsHandler,
			Interaction:  interactHandler,
			Moderation:   modHandler,
			Notification: notifHandler,
		},
		notifications: deps.NotificationStore,
	}
}

// RegisterRoutes adds community endpoints to the application router.
func (m *Module) RegisterRoutes(router gin.IRouter) { m.routes.RegisterRoutes(router) }
