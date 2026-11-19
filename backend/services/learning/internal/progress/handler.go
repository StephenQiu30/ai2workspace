package progress

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SaveProgressRequest defines payload to update student study position.
type SaveProgressRequest struct {
	CourseID        string `json:"courseId" binding:"required"`
	ContentVersion  int    `json:"contentVersion" binding:"required"`
	PositionSeconds int    `json:"positionSeconds"`
	ExpectedVersion int    `json:"expectedVersion"`
}

// ProgressResponse represents serialized progress record.
type ProgressResponse struct {
	ID              string `json:"id"`
	UserID          string `json:"userId"`
	CourseID        string `json:"courseId"`
	LessonID        string `json:"lessonId"`
	ContentVersion  int    `json:"contentVersion"`
	PositionSeconds int    `json:"positionSeconds"`
	Version         int    `json:"version"`
	UpdatedAt       string `json:"updatedAt"`
}

// Handler manages progress HTTP endpoints.
type Handler struct {
	store Store
}

// NewHandler creates a progress Handler.
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// SaveProgress updates or creates study progress for a lesson.
// @ID learningSaveProgress
// @Summary Save learning progress
// @Description Updates current study timestamp for lesson with optimistic version check.
// @Tags Progress
// @Accept json
// @Produce json
// @Param id path string true "Lesson ID"
// @Param request body SaveProgressRequest true "Progress update payload"
// @Success 200 {object} ProgressResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/learning/lessons/{id}/progress [put]
func (h *Handler) SaveProgress(c *gin.Context) {
	lessonID := c.Param("id")
	var req SaveProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "user_mock"
	}

	p, err := h.store.SaveProgress(c.Request.Context(), userID, req.CourseID, lessonID, req.ContentVersion, req.PositionSeconds, req.ExpectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, progressToResponse(p))
}

// GetProgress returns the user's latest progress in a lesson.
// @ID learningGetProgress
// @Summary Get learning progress for lesson
// @Tags Progress
// @Produce json
// @Param id path string true "Lesson ID"
// @Success 200 {object} ProgressResponse
// @Failure 404 {object} map[string]string
// @Router /api/learning/lessons/{id}/progress [get]
func (h *Handler) GetProgress(c *gin.Context) {
	lessonID := c.Param("id")
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "user_mock"
	}

	p, err := h.store.GetProgress(c.Request.Context(), userID, lessonID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, progressToResponse(p))
}

// ListCourseProgress returns all user progress records for a course.
// @ID learningListCourseProgress
// @Summary List learning progress for all lessons in a course
// @Tags Progress
// @Produce json
// @Param id path string true "Course ID"
// @Success 200 {array} ProgressResponse
// @Router /api/learning/courses/{id}/progress [get]
func (h *Handler) ListCourseProgress(c *gin.Context) {
	courseID := c.Param("id")
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "user_mock"
	}

	list, err := h.store.ListProgressByCourse(c.Request.Context(), userID, courseID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := make([]ProgressResponse, 0, len(list))
	for _, p := range list {
		resp = append(resp, progressToResponse(p))
	}
	c.JSON(http.StatusOK, resp)
}

func progressToResponse(p *Progress) ProgressResponse {
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
