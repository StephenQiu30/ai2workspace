// Package learning owns courses, media and learning progress.
package learning

import (
	"context"
	"errors"
	"net/http"

	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	httpapi "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http"
	authhttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/auth"
	courseshttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/courses"
	mediahttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/media"
	progresshttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/progress"
	coursespg "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/postgres/courses"
	mediapg "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/postgres/media"
	progresspg "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/postgres/progress"
	"github.com/StephenQiu30/ai2workspace/backend/internal/learning/application"
	coursesapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/courses"
	mediaapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/media"
	progressapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/progress"
	coursesdomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/courses"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps holds explicit dependencies used to construct the learning HTTP router.
// Integration tests may inject a custom Authenticator or Entitlements client here;
// production defaults never trust plaintext identity headers.
type Deps struct {
	PlaybackSigningSecret string
	CoursesStore          coursesapp.Store
	MediaStore            mediaapp.Store
	ProgressStore         progressapp.Store
	Authenticator         authhttp.Authenticator
	Entitlements          coursesapp.EntitlementClient
}

// SaleSnapshot is the course state exposed to other modules.
type SaleSnapshot = application.SaleSnapshot

// Module wires learning operations and HTTP endpoints.
type Module struct {
	*application.Catalog
	routes httpapi.Routes
}

// New builds learning with PostgreSQL storage and the shared session resolver.
func New(pool *pgxpool.Pool, resolve identity.SessionResolver, grants coursesapp.EntitlementClient, signingSecret string) *Module {
	return NewWithDeps(Deps{
		PlaybackSigningSecret: signingSecret,
		CoursesStore:          coursespg.NewPostgresStore(pool),
		MediaStore:            mediapg.NewPostgresStore(pool),
		ProgressStore:         progresspg.NewPostgresStore(pool),
		Entitlements:          grants,
		Authenticator: func(ctx context.Context, r *http.Request) (*authhttp.Principal, error) {
			cookie, err := r.Cookie(identity.SessionCookieName)
			if err != nil || cookie.Value == "" {
				return nil, nil
			}
			actor, err := resolve(ctx, cookie.Value)
			switch {
			case errors.Is(err, identity.ErrInvalidSession):
				return nil, authhttp.ErrUnauthenticated
			case errors.Is(err, identity.ErrInactiveAccount):
				return nil, authhttp.ErrAccountSuspended
			case err != nil:
				return nil, authhttp.ErrAuthUnavailable
			}
			return &authhttp.Principal{
				UserID:        actor.ID,
				Email:         actor.Email,
				Nickname:      actor.Nickname,
				EmailVerified: actor.EmailVerified,
				Roles:         actor.Roles,
				Status:        actor.Status,
			}, nil
		},
	})
}

// NewWithDeps builds learning with explicit dependencies.
func NewWithDeps(deps Deps) *Module {
	signingSecret := deps.PlaybackSigningSecret

	coursesStore := deps.CoursesStore
	if coursesStore == nil {
		coursesStore = coursesapp.NewMemoryStore()
	}
	mediaStore := deps.MediaStore
	if mediaStore == nil {
		mediaStore = mediaapp.NewMemoryStore()
	}
	progressStore := deps.ProgressStore
	if progressStore == nil {
		progressStore = progressapp.NewMemoryStore()
	}

	authenticator := deps.Authenticator
	if authenticator == nil {
		authenticator = func(context.Context, *http.Request) (*authhttp.Principal, error) { return nil, nil }
	}

	coursesHandler := courseshttp.NewHandler(coursesStore, deps.Entitlements, func(ctx context.Context, userID string, course *coursesdomain.Course) (coursesapp.CourseProgressSummary, error) {
		return application.SummarizeProgress(ctx, progressStore, mediaStore, userID, course)
	})

	lessonContextResolver := func(ctx context.Context, lessonID string) (*mediahttp.LessonContext, error) {
		lesson, course, err := coursesStore.GetLesson(ctx, lessonID)
		if err != nil {
			return nil, err
		}
		return &mediahttp.LessonContext{
			LessonID:         lesson.ID,
			CourseID:         course.ID,
			OwnerID:          course.OwnerID,
			IsFree:           course.IsFree,
			IsTrial:          lesson.IsTrial,
			ProductionState:  course.ProductionState,
			DeliveryState:    course.DeliveryState,
			PublishedVersion: course.PublishedVersion,
			MediaVersionID:   lesson.MediaVersionID,
		}, nil
	}

	mediaAccessChecker := func(ctx context.Context, p *authhttp.Principal, courseID string) (bool, string, error) {
		course, err := coursesStore.GetCourse(ctx, courseID)
		if err != nil {
			return false, "course_not_found", nil
		}
		return coursesapp.CheckUserCourseAccess(ctx, coursesStore, deps.Entitlements, p.UserID, p.Roles, course, "playback")
	}

	mediaHandler := mediahttp.NewHandler(
		mediaStore,
		lessonContextResolver,
		mediaAccessChecker,
		coursesStore.SetLessonMediaVersion,
		signingSecret,
	)

	progressLessonResolver := func(ctx context.Context, lessonID string) (string, error) {
		lesson, _, err := coursesStore.GetLesson(ctx, lessonID)
		if err != nil {
			return "", progresshttp.ErrCourseNotFound
		}
		return lesson.CourseID, nil
	}

	progressAccessChecker := func(ctx context.Context, p *authhttp.Principal, courseID string) (bool, string, error) {
		course, err := coursesStore.GetCourse(ctx, courseID)
		if err != nil {
			if errors.Is(err, coursesdomain.ErrCourseNotFound) {
				return false, "course_not_found", progresshttp.ErrCourseNotFound
			}
			return false, "internal_error", err
		}
		return coursesapp.CheckUserCourseAccess(ctx, coursesStore, deps.Entitlements, p.UserID, p.Roles, course, "playback")
	}

	progressHandler := progresshttp.NewHandler(progressStore, progressLessonResolver, progressAccessChecker)

	return &Module{
		Catalog: application.NewCatalog(coursesStore, deps.Entitlements),
		routes: httpapi.Routes{
			Authenticator: authenticator,
			Courses:       coursesHandler,
			Media:         mediaHandler,
			Progress:      progressHandler,
		},
	}
}

// RegisterRoutes adds learning endpoints to the application router.
func (m *Module) RegisterRoutes(router gin.IRouter) { m.routes.RegisterRoutes(router) }
