package progress

import (
	"context"
	"errors"
	"net/http"

	authhttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/auth"
	progressapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/progress"
	progressdomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/progress"
	"github.com/gin-gonic/gin"
)

// ErrCourseNotFound indicates the target course or lesson does not exist.
var ErrCourseNotFound = errors.New("course or lesson not found")

// LessonCourseResolver returns the course ID owning lessonID, or ErrCourseNotFound if missing.
type LessonCourseResolver func(ctx context.Context, lessonID string) (courseID string, err error)

// CourseAccessChecker verifies whether the authenticated principal currently holds access to courseID.
// Returns (allowed, reason, err); err == ErrCourseNotFound if course does not exist.
type CourseAccessChecker func(ctx context.Context, p *authhttp.Principal, courseID string) (bool, string, error)

// Handler manages progress HTTP endpoints.
type Handler struct {
	store          progressapp.Store
	lessonResolver LessonCourseResolver
	accessChecker  CourseAccessChecker
}

// NewHandler creates a progress Handler.
func NewHandler(store progressapp.Store, lessonResolver LessonCourseResolver, accessChecker CourseAccessChecker) *Handler {
	return &Handler{
		store:          store,
		lessonResolver: lessonResolver,
		accessChecker:  accessChecker,
	}
}

func (h *Handler) checkAccess(c *gin.Context, p *authhttp.Principal, courseID string) bool {
	if h.accessChecker == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "Access verification unavailable"})
		return false
	}
	allowed, reason, err := h.accessChecker(c.Request.Context(), p, courseID)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "course_not_found", "message": "Course not found"})
			return false
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "entitlement_service_unavailable", "message": "Failed to verify course access"})
		return false
	}
	if !allowed {
		if reason == "" {
			reason = "access_denied"
		}
		c.JSON(http.StatusForbidden, gin.H{"error": reason, "message": "You do not have access to this course"})
		return false
	}
	return true
}

// SaveProgress updates or creates study progress for a lesson.
// @ID learningSaveProgress
// @Summary Save learning progress
// @Description Updates current study timestamp for a lesson with course access verification and strict optimistic version check.
// @Tags Progress
// @Accept json
// @Produce json
// @Param id path string true "Lesson ID"
// @Param request body SaveProgressRequest true "Progress update payload"
// @Success 200 {object} ProgressResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]interface{}
// @Failure 503 {object} map[string]string
// @Router /api/learning/lessons/{id}/progress [put]
func (h *Handler) SaveProgress(c *gin.Context) {
	p, ok := authhttp.RequirePrincipal(c)
	if !ok {
		return
	}

	lessonID := c.Param("id")
	var req SaveProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	if h.lessonResolver != nil {
		courseID, err := h.lessonResolver(c.Request.Context(), lessonID)
		if err != nil || courseID != req.CourseID {
			c.JSON(http.StatusNotFound, gin.H{"error": "lesson_not_found", "message": "Lesson not found in specified course"})
			return
		}
	}

	if !h.checkAccess(c, p, req.CourseID) {
		return
	}

	record, err := h.store.SaveProgress(c.Request.Context(), p.UserID, req.CourseID, lessonID, req.ContentVersion, req.PositionSeconds, req.ExpectedVersion)
	if err != nil {
		if errors.Is(err, progressdomain.ErrVersionConflict) {
			resp := gin.H{"error": "version_conflict", "message": err.Error()}
			if current, getErr := h.store.GetProgress(c.Request.Context(), p.UserID, lessonID); getErr == nil {
				resp["current"] = progressToResponse(current)
			}
			c.JSON(http.StatusConflict, resp)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_progress", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, progressToResponse(record))
}

// GetProgress returns the user's latest progress in a lesson.
// @ID learningGetProgress
// @Summary Get learning progress for lesson
// @Tags Progress
// @Produce json
// @Param id path string true "Lesson ID"
// @Success 200 {object} ProgressResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /api/learning/lessons/{id}/progress [get]
func (h *Handler) GetProgress(c *gin.Context) {
	p, ok := authhttp.RequirePrincipal(c)
	if !ok {
		return
	}

	lessonID := c.Param("id")
	if h.lessonResolver != nil {
		courseID, err := h.lessonResolver(c.Request.Context(), lessonID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "lesson_not_found", "message": "Lesson not found"})
			return
		}
		if !h.checkAccess(c, p, courseID) {
			return
		}
	}

	record, err := h.store.GetProgress(c.Request.Context(), p.UserID, lessonID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "progress_not_found", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, progressToResponse(record))
}

// ListCourseProgress returns all user progress records for a course.
// @ID learningListCourseProgress
// @Summary List learning progress for all lessons in a course
// @Tags Progress
// @Produce json
// @Param id path string true "Course ID"
// @Success 200 {array} ProgressResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /api/learning/courses/{id}/progress [get]
func (h *Handler) ListCourseProgress(c *gin.Context) {
	p, ok := authhttp.RequirePrincipal(c)
	if !ok {
		return
	}

	courseID := c.Param("id")
	if !h.checkAccess(c, p, courseID) {
		return
	}

	list, err := h.store.ListProgressByCourse(c.Request.Context(), p.UserID, courseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}

	resp := make([]ProgressResponse, 0, len(list))
	for _, item := range list {
		resp = append(resp, progressToResponse(item))
	}
	c.JSON(http.StatusOK, resp)
}

func progressToResponse(p *progressdomain.Progress) ProgressResponse {
	return ProgressResponse{
		ID:              p.ID,
		UserID:          p.UserID,
		CourseID:        p.CourseID,
		LessonID:        p.LessonID,
		ContentVersion:  p.ContentVersion,
		PositionSeconds: p.PositionSeconds,
		Version:         p.Version,
		UpdatedAt:       p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
