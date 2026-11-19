// Package server assembles the learning service's HTTP routes.
package server

import (
	"net/http"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/learning/api"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/courses"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/health"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// New returns an independent router with default in-memory store.
func New(swaggerEnabled bool) *gin.Engine {
	store := courses.NewMemoryStore()
	return NewWithStore(swaggerEnabled, store, "identity-service-secret")
}

// NewWithStore returns a router wired with a specific store and service configuration.
func NewWithStore(swaggerEnabled bool, store courses.Store, serviceToken string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	coursesHandler := courses.NewHandler(store, serviceToken)

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
