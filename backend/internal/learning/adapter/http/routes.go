// Package http binds endpoints for the learning module.
package http

import (
	authhttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/auth"
	courseshttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/courses"
	mediahttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/media"
	progresshttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/progress"
	"github.com/gin-gonic/gin"
)

// Routes holds the HTTP handlers for learning.
type Routes struct {
	Authenticator authhttp.Authenticator
	Courses       *courseshttp.Handler
	Media         *mediahttp.Handler
	Progress      *progresshttp.Handler
}

// RegisterRoutes adds learning endpoints to the application router.
func (r Routes) RegisterRoutes(router gin.IRouter) {
	router = router.Group("", authhttp.Middleware(r.Authenticator))

	// Course Discovery & Management Endpoints
	router.GET("/api/learning/courses", r.Courses.ListCourses)
	router.GET("/api/learning/manage/courses", r.Courses.ListManageCourses)
	router.GET("/api/learning/courses/:id", r.Courses.GetCourse)
	router.POST("/api/learning/courses", r.Courses.CreateCourse)
	router.PATCH("/api/learning/courses/:id", r.Courses.UpdateCourse)
	router.DELETE("/api/learning/courses/:id", r.Courses.DeleteCourse)
	router.POST("/api/learning/courses/:id/submissions", r.Courses.SubmitCourse)
	router.POST("/api/learning/courses/:id/reviews", r.Courses.ReviewCourse)
	router.POST("/api/learning/courses/:id/sales-closure", r.Courses.CloseSales)
	router.POST("/api/learning/courses/:id/withdrawals", r.Courses.WithdrawCourse)
	router.POST("/api/learning/courses/:id/sections", r.Courses.AddSection)
	router.PATCH("/api/learning/courses/:id/sections/:sectionId", r.Courses.UpdateSection)
	router.DELETE("/api/learning/courses/:id/sections/:sectionId", r.Courses.DeleteSection)
	router.POST("/api/learning/courses/:id/lessons", r.Courses.AddLesson)
	router.PATCH("/api/learning/courses/:id/lessons/:lessonId", r.Courses.UpdateLesson)
	router.DELETE("/api/learning/courses/:id/lessons/:lessonId", r.Courses.DeleteLesson)
	router.POST("/api/learning/courses/:id/enrollments", r.Courses.JoinFreeCourse)
	router.GET("/api/learning/me/courses", r.Courses.ListMyCourses)

	// Media Endpoints
	router.POST("/api/learning/lessons/:id/uploads", r.Media.CreateUpload)
	router.POST("/api/learning/uploads/:id/completion", r.Media.CompleteUpload)
	router.GET("/api/learning/uploads/:id", r.Media.GetUpload)
	router.DELETE("/api/learning/uploads/:id", r.Media.CancelUpload)
	router.POST("/api/learning/lessons/:id/media-publication", r.Media.PublishLessonMedia)
	router.POST("/api/learning/media-jobs/:id/retries", r.Media.RetryMediaJob)
	router.POST("/api/learning/lessons/:id/playback-authorizations", r.Media.AuthorizePlayback)

	// Progress Endpoints
	router.PUT("/api/learning/lessons/:id/progress", r.Progress.SaveProgress)
	router.GET("/api/learning/lessons/:id/progress", r.Progress.GetProgress)
	router.GET("/api/learning/courses/:id/progress", r.Progress.ListCourseProgress)
}
