// Package server assembles the learning service's HTTP routes and dependencies.
package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/learning/api"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/courses"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/health"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/media"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/progress"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Deps holds explicit dependencies used to construct the learning HTTP router.
// Integration tests may inject a custom Authenticator or Entitlements client here;
// production defaults never trust plaintext identity headers.
type Deps struct {
	SwaggerEnabled        bool
	ServiceToken          string
	PlaybackSigningSecret string
	CoursesStore          courses.Store
	MediaStore            media.Store
	ProgressStore         progress.Store
	Authenticator         auth.Authenticator
	Entitlements          courses.EntitlementClient
}

// New returns an independent router with default in-memory stores and fail-closed auth.
func New(swaggerEnabled bool) *gin.Engine {
	return NewWithDeps(Deps{SwaggerEnabled: swaggerEnabled})
}

// NewFromConfig assembles the production router from process configuration and stores.
func NewFromConfig(cfg config.Config, coursesStore courses.Store, mediaStore media.Store, progressStore progress.Store) *gin.Engine {
	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	return NewWithDeps(Deps{
		SwaggerEnabled:        cfg.SwaggerEnabled,
		ServiceToken:          cfg.ServiceToken,
		PlaybackSigningSecret: cfg.PlaybackSigningSecret,
		CoursesStore:          coursesStore,
		MediaStore:            mediaStore,
		ProgressStore:         progressStore,
		Authenticator:         NewIdentityAuthenticator(cfg.IdentityServiceURL, cfg.ServiceToken, httpClient),
		Entitlements:          NewCommerceClient(cfg.CommerceServiceURL, cfg.ServiceToken, httpClient),
	})
}

// NewWithDeps returns a fully wired router for courses, media, and progress.
func NewWithDeps(deps Deps) *gin.Engine {
	serviceToken := deps.ServiceToken
	if serviceToken == "" {
		serviceToken = "identity-service-secret"
	}
	signingSecret := deps.PlaybackSigningSecret
	if signingSecret == "" {
		signingSecret = serviceToken
	}

	coursesStore := deps.CoursesStore
	if coursesStore == nil {
		coursesStore = courses.NewMemoryStore()
	}
	mediaStore := deps.MediaStore
	if mediaStore == nil {
		mediaStore = media.NewMemoryStore()
	}
	progressStore := deps.ProgressStore
	if progressStore == nil {
		progressStore = progress.NewMemoryStore()
	}

	authenticator := deps.Authenticator
	if authenticator == nil {
		authenticator = NewIdentityAuthenticator("", serviceToken, nil)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(auth.Middleware(authenticator))

	progressSummaryFn := func(ctx context.Context, userID string, course *courses.Course) (courses.CourseProgressSummary, error) {
		summary := courses.CourseProgressSummary{
			TotalLessonCount: course.LessonCount,
		}
		records, err := progressStore.ListProgressByCourse(ctx, userID, course.ID)
		if err != nil {
			return summary, err
		}

		lessonTypes := make(map[string]string, course.LessonCount)
		lessonMediaIDs := make(map[string]string, course.LessonCount)
		for _, sec := range course.Sections {
			for _, l := range sec.Lessons {
				lessonTypes[l.ID] = l.Type
				lessonMediaIDs[l.ID] = l.MediaVersionID
			}
		}

		var latestTime *time.Time
		watchedSet := make(map[string]struct{}, len(records))
		completedSet := make(map[string]struct{}, len(records))
		for _, rec := range records {
			watchedSet[rec.LessonID] = struct{}{}
			if latestTime == nil || rec.UpdatedAt.After(*latestTime) {
				t := rec.UpdatedAt
				latestTime = &t
				summary.LastLessonID = rec.LessonID
				summary.LastPositionSeconds = rec.PositionSeconds
			}

			lType := lessonTypes[rec.LessonID]
			if lType == "article" {
				completedSet[rec.LessonID] = struct{}{}
			} else if rec.PositionSeconds > 0 {
				var mv *media.MediaVersion
				if mvID := lessonMediaIDs[rec.LessonID]; mvID != "" {
					mv, _ = mediaStore.GetMediaVersion(ctx, mvID)
				}
				if mv == nil {
					mv, _ = mediaStore.GetLessonMediaVersion(ctx, rec.LessonID)
				}
				if mv != nil && mv.DurationSeconds > 0 && rec.PositionSeconds >= mv.DurationSeconds {
					completedSet[rec.LessonID] = struct{}{}
				}
			}
		}

		summary.WatchedLessonCount = len(watchedSet)
		summary.CompletedLessonCount = len(completedSet)
		summary.LastStudiedAt = latestTime
		return summary, nil
	}

	coursesHandler := courses.NewHandler(coursesStore, serviceToken, deps.Entitlements, progressSummaryFn)

	lessonContextResolver := func(ctx context.Context, lessonID string) (*media.LessonContext, error) {
		lesson, course, err := coursesStore.GetLesson(ctx, lessonID)
		if err != nil {
			return nil, err
		}
		return &media.LessonContext{
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

	mediaAccessChecker := func(ctx context.Context, p *auth.Principal, courseID string) (bool, string, error) {
		course, err := coursesStore.GetCourse(ctx, courseID)
		if err != nil {
			return false, "course_not_found", nil
		}
		return coursesHandler.CheckUserCourseAccess(ctx, p.UserID, p.Roles, course, "playback")
	}

	mediaHandler := media.NewHandler(
		mediaStore,
		lessonContextResolver,
		mediaAccessChecker,
		coursesStore.SetLessonMediaVersion,
		signingSecret,
	)

	progressLessonResolver := func(ctx context.Context, lessonID string) (string, error) {
		lesson, _, err := coursesStore.GetLesson(ctx, lessonID)
		if err != nil {
			return "", progress.ErrCourseNotFound
		}
		return lesson.CourseID, nil
	}

	progressAccessChecker := func(ctx context.Context, p *auth.Principal, courseID string) (bool, string, error) {
		course, err := coursesStore.GetCourse(ctx, courseID)
		if err != nil {
			if errors.Is(err, courses.ErrCourseNotFound) {
				return false, "course_not_found", progress.ErrCourseNotFound
			}
			return false, "internal_error", err
		}
		return coursesHandler.CheckUserCourseAccess(ctx, p.UserID, p.Roles, course, "playback")
	}

	progressHandler := progress.NewHandler(progressStore, progressLessonResolver, progressAccessChecker)

	// Health
	router.GET("/api/learning/health", health.Get)

	// Course Discovery & Management Endpoints
	router.GET("/api/learning/courses", coursesHandler.ListCourses)
	router.GET("/api/learning/manage/courses", coursesHandler.ListManageCourses)
	router.GET("/api/learning/courses/:id", coursesHandler.GetCourse)
	router.POST("/api/learning/courses", coursesHandler.CreateCourse)
	router.PATCH("/api/learning/courses/:id", coursesHandler.UpdateCourse)
	router.DELETE("/api/learning/courses/:id", coursesHandler.DeleteCourse)
	router.POST("/api/learning/courses/:id/submissions", coursesHandler.SubmitCourse)
	router.POST("/api/learning/courses/:id/reviews", coursesHandler.ReviewCourse)
	router.POST("/api/learning/courses/:id/sales-closure", coursesHandler.CloseSales)
	router.POST("/api/learning/courses/:id/withdrawals", coursesHandler.WithdrawCourse)
	router.POST("/api/learning/courses/:id/sections", coursesHandler.AddSection)
	router.PATCH("/api/learning/courses/:id/sections/:sectionId", coursesHandler.UpdateSection)
	router.DELETE("/api/learning/courses/:id/sections/:sectionId", coursesHandler.DeleteSection)
	router.POST("/api/learning/courses/:id/lessons", coursesHandler.AddLesson)
	router.PATCH("/api/learning/courses/:id/lessons/:lessonId", coursesHandler.UpdateLesson)
	router.DELETE("/api/learning/courses/:id/lessons/:lessonId", coursesHandler.DeleteLesson)
	router.POST("/api/learning/courses/:id/enrollments", coursesHandler.JoinFreeCourse)
	router.GET("/api/learning/me/courses", coursesHandler.ListMyCourses)

	// Media Endpoints
	router.POST("/api/learning/lessons/:id/uploads", mediaHandler.CreateUpload)
	router.POST("/api/learning/uploads/:id/completion", mediaHandler.CompleteUpload)
	router.GET("/api/learning/uploads/:id", mediaHandler.GetUpload)
	router.DELETE("/api/learning/uploads/:id", mediaHandler.CancelUpload)
	router.POST("/api/learning/lessons/:id/media-publication", mediaHandler.PublishLessonMedia)
	router.POST("/api/learning/media-jobs/:id/retries", mediaHandler.RetryMediaJob)
	router.POST("/api/learning/lessons/:id/playback-authorizations", mediaHandler.AuthorizePlayback)

	// Progress Endpoints
	router.PUT("/api/learning/lessons/:id/progress", progressHandler.SaveProgress)
	router.GET("/api/learning/lessons/:id/progress", progressHandler.GetProgress)
	router.GET("/api/learning/courses/:id/progress", progressHandler.ListCourseProgress)

	// Internal Integration Endpoints
	router.GET("/internal/learning/courses/:courseId/sale-snapshot", coursesHandler.GetSaleSnapshot)
	router.POST("/internal/learning/course-access/check", coursesHandler.CheckCourseAccess)
	router.POST("/internal/learning/course-management/check", coursesHandler.CheckCourseManagement)
	router.GET("/internal/learning/courses/:courseId/delivery-evidence", coursesHandler.GetDeliveryEvidence)

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	})

	if deps.SwaggerEnabled {
		router.GET("/swagger/learning/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	return router
}
