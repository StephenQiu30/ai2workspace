// Package server assembles the learning service's HTTP routes.
package server

import (
	"net/http"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/learning/api"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/courses"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/health"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/media"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/progress"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// New returns an independent router with default in-memory store.
func New(swaggerEnabled bool) *gin.Engine {
	coursesStore := courses.NewMemoryStore()
	mediaStore := media.NewMemoryStore(func(courseID, instructorID string) (bool, error) {
		course, err := coursesStore.GetCourse(nil, courseID)
		if err != nil {
			return false, err
		}
		return course.OwnerID == instructorID, nil
	})
	progressStore := progress.NewMemoryStore()
	return NewWithStores(swaggerEnabled, coursesStore, mediaStore, progressStore, "identity-service-secret")
}

// NewWithStore returns a router wired with a specific store and service configuration (backward compatible).
func NewWithStore(swaggerEnabled bool, store courses.Store, serviceToken string) *gin.Engine {
	mediaStore := media.NewMemoryStore(nil)
	progressStore := progress.NewMemoryStore()
	return NewWithStores(swaggerEnabled, store, mediaStore, progressStore, serviceToken)
}

// NewWithStores returns a fully wired router for courses, media, and progress.
func NewWithStores(swaggerEnabled bool, coursesStore courses.Store, mediaStore media.Store, progressStore progress.Store, serviceToken string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	coursesHandler := courses.NewHandler(coursesStore, serviceToken)
	mediaHandler := media.NewHandler(mediaStore, func(courseID, lessonID, userID string, isTrial bool) (bool, error) {
		// Mock access checker: trial allowed, free course allowed, otherwise allows valid user
		if isTrial {
			return true, nil
		}
		course, err := coursesStore.GetCourse(nil, courseID)
		if err != nil {
			return false, err
		}
		if course.IsFree {
			return true, nil
		}
		return userID != "" && userID != "anonymous", nil
	})
	progressHandler := progress.NewHandler(progressStore)

	// Health
	router.GET("/api/learning/health", health.Get)

	// Public Course Endpoints
	router.GET("/api/learning/courses", coursesHandler.ListCourses)
	router.GET("/api/learning/courses/:id", coursesHandler.GetCourse)
	router.POST("/api/learning/courses", coursesHandler.CreateCourse)
	router.PATCH("/api/learning/courses/:id", coursesHandler.UpdateCourse)
	router.DELETE("/api/learning/courses/:id", coursesHandler.DeleteCourse)
	router.POST("/api/learning/courses/:id/submissions", coursesHandler.SubmitCourse)
	router.POST("/api/learning/courses/:id/reviews", coursesHandler.ReviewCourse)
	router.POST("/api/learning/courses/:id/sales-closure", coursesHandler.CloseSales)
	router.POST("/api/learning/courses/:id/withdrawals", coursesHandler.WithdrawCourse)
	router.POST("/api/learning/courses/:id/sections", coursesHandler.AddSection)
	router.POST("/api/learning/courses/:id/lessons", coursesHandler.AddLesson)
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

	if swaggerEnabled {
		router.GET("/swagger/learning/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	return router
}
