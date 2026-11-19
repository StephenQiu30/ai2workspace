package media

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// CourseAccessChecker checks whether a user has playback access to a course.
type CourseAccessChecker func(courseID, lessonID, userID string, isTrial bool) (bool, error)

// Handler handles media-related HTTP endpoints.
type Handler struct {
	store         Store
	accessChecker CourseAccessChecker
}

// NewHandler creates a new media Handler.
func NewHandler(store Store, checker CourseAccessChecker) *Handler {
	return &Handler{
		store:         store,
		accessChecker: checker,
	}
}

// CreateUpload initiates a controlled media upload session.
// @ID learningCreateUpload
// @Summary Initiate direct video upload
// @Description Creates a short-lived direct upload slot enforcing MP4, <=2GiB, max 2 concurrent uploads.
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Lesson ID"
// @Param request body CreateUploadRequest true "Upload metadata"
// @Success 201 {object} UploadSessionResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 413 {object} map[string]string
// @Failure 429 {object} map[string]string
// @Router /api/learning/lessons/{id}/uploads [post]
func (h *Handler) CreateUpload(c *gin.Context) {
	lessonID := c.Param("id")
	var req CreateUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	instructorID := c.GetHeader("X-User-ID")
	if instructorID == "" {
		instructorID = "instructor_mock"
	}

	session, err := h.store.CreateUpload(c.Request.Context(), instructorID, req.CourseID, lessonID, req.FileName, req.DeclaredBytes)
	if err != nil {
		switch {
		case errors.Is(err, ErrFileTooLarge):
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
		case errors.Is(err, ErrMaxConcurrentUploads):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, sessionToResponse(session))
}

// CompleteUpload verifies uploaded media and creates a ready/failed media version.
// @ID learningCompleteUpload
// @Summary Complete media upload and trigger validation/packaging
// @Description Validates MP4, H.264, duration <=120m, long side <=1920, short side <=1080, fps <=30.
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Upload ID"
// @Param request body CompleteUploadRequest true "Probed media data"
// @Success 200 {object} MediaVersionResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 410 {object} map[string]string
// @Router /api/learning/uploads/{id}/completion [post]
func (h *Handler) CompleteUpload(c *gin.Context) {
	uploadID := c.Param("id")
	var req CompleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	meta := MediaMetadata{
		Container:       req.Container,
		Codec:           req.Codec,
		DurationSeconds: req.DurationSeconds,
		Width:           req.Width,
		Height:          req.Height,
		FPS:             req.FPS,
		ActualBytes:     req.ActualBytes,
	}

	version, err := h.store.CompleteUpload(c.Request.Context(), uploadID, req.IdempotencyKey, meta)
	if err != nil {
		switch {
		case errors.Is(err, ErrUploadNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUploadExpired), errors.Is(err, ErrUploadCancelled):
			c.JSON(http.StatusGone, gin.H{"error": err.Error()})
		default:
			// Returns 422 Unprocessable Entity or 400 Bad Request with failed media version
			if version != nil {
				c.JSON(http.StatusUnprocessableEntity, versionToResponse(version))
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, versionToResponse(version))
}

// GetUpload returns the current state of an upload session.
// @ID learningGetUpload
// @Summary Get upload session status
// @Tags Media
// @Produce json
// @Param id path string true "Upload ID"
// @Success 200 {object} UploadSessionResponse
// @Failure 404 {object} map[string]string
// @Router /api/learning/uploads/{id} [get]
func (h *Handler) GetUpload(c *gin.Context) {
	uploadID := c.Param("id")
	session, err := h.store.GetUpload(c.Request.Context(), uploadID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sessionToResponse(session))
}

// CancelUpload aborts an active upload session.
// @ID learningCancelUpload
// @Summary Cancel active upload session
// @Tags Media
// @Produce json
// @Param id path string true "Upload ID"
// @Success 204
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/learning/uploads/{id} [delete]
func (h *Handler) CancelUpload(c *gin.Context) {
	uploadID := c.Param("id")
	instructorID := c.GetHeader("X-User-ID")
	if instructorID == "" {
		instructorID = "instructor_mock"
	}

	if err := h.store.CancelUpload(c.Request.Context(), uploadID, instructorID); err != nil {
		switch {
		case errors.Is(err, ErrUploadNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// PublishLessonMedia binds a ready media version to a lesson.
// @ID learningPublishLessonMedia
// @Summary Publish ready media to lesson
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Lesson ID"
// @Param request body PublishMediaRequest true "Media Version"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/learning/lessons/{id}/media-publication [post]
func (h *Handler) PublishLessonMedia(c *gin.Context) {
	lessonID := c.Param("id")
	var req PublishMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	instructorID := c.GetHeader("X-User-ID")
	if instructorID == "" {
		instructorID = "instructor_mock"
	}

	if err := h.store.PublishLessonMedia(c.Request.Context(), lessonID, req.MediaVersionID, instructorID); err != nil {
		switch {
		case errors.Is(err, ErrMediaVersionNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrMediaNotReady):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "published", "mediaVersionId": req.MediaVersionID})
}

// RetryMediaJob retries a failed asynchronous transcoding job.
// @ID learningRetryMediaJob
// @Summary Retry failed media processing job
// @Tags Media
// @Produce json
// @Param id path string true "Job ID"
// @Success 200 {object} MediaJobResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/learning/media-jobs/{id}/retries [post]
func (h *Handler) RetryMediaJob(c *gin.Context) {
	jobID := c.Param("id")
	instructorID := c.GetHeader("X-User-ID")
	if instructorID == "" {
		instructorID = "instructor_mock"
	}

	job, err := h.store.RetryMediaJob(c.Request.Context(), jobID, instructorID)
	if err != nil {
		switch {
		case errors.Is(err, ErrMediaJobNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, MediaJobResponse{
		ID:             job.ID,
		UploadID:       job.UploadID,
		MediaVersionID: job.MediaVersionID,
		Status:         job.Status,
		RetryCount:     job.RetryCount,
		ErrorMessage:   job.ErrorMessage,
		UpdatedAt:      job.UpdatedAt,
	})
}

// AuthorizePlayback issues a short-lived playback token for authorized users.
// @ID learningAuthorizePlayback
// @Summary Authorize HLS playback
// @Description Checks course state and user entitlement before issuing short-lived HLS token.
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Lesson ID"
// @Param request body AuthorizePlaybackRequest true "Playback Request"
// @Success 200 {object} PlaybackAuthorizationResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/learning/lessons/{id}/playback-authorizations [post]
func (h *Handler) AuthorizePlayback(c *gin.Context) {
	lessonID := c.Param("id")
	var req AuthorizePlaybackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "anonymous"
	}

	// 1. Get lesson published media version
	mediaVer, err := h.store.GetLessonMediaVersion(c.Request.Context(), lessonID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "lesson media version not found or not published"})
		return
	}
	if mediaVer.Status != MediaReady {
		c.JSON(http.StatusConflict, gin.H{"error": "media is not ready for playback"})
		return
	}

	// 2. Check access entitlement
	isTrial := req.Purpose == "trial"
	if h.accessChecker != nil {
		allowed, err := h.accessChecker(req.CourseID, lessonID, userID, isTrial)
		if err != nil || !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied: playback not entitled"})
			return
		}
	}

	// 3. Issue short-lived playback token (valid 2 hours)
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(2 * time.Hour)

	c.JSON(http.StatusOK, PlaybackAuthorizationResponse{
		MediaVersionID: mediaVer.ID,
		PlaybackURL:    fmt.Sprintf("%s?token=%s", mediaVer.PlaybackURL, token),
		HLSQualities:   mediaVer.HLSQualities,
		ExpiresAt:      expiresAt,
		Token:          token,
	})
}

func sessionToResponse(s *UploadSession) UploadSessionResponse {
	return UploadSessionResponse{
		ID:                 s.ID,
		InstructorID:       s.InstructorID,
		CourseID:           s.CourseID,
		LessonID:           s.LessonID,
		FileName:           s.FileName,
		DeclaredBytes:      s.DeclaredBytes,
		ActualBytes:        s.ActualBytes,
		Status:             string(s.Status),
		SourceVersion:      s.SourceVersion,
		ExpiresAt:          s.ExpiresAt,
		UploadInstructions: s.UploadInstructions,
		MediaVersionID:     s.MediaVersionID,
	}
}

func versionToResponse(v *MediaVersion) MediaVersionResponse {
	return MediaVersionResponse{
		ID:              v.ID,
		UploadID:        v.UploadID,
		LessonID:        v.LessonID,
		Status:          string(v.Status),
		DurationSeconds: v.DurationSeconds,
		Width:           v.Width,
		Height:          v.Height,
		FPS:             v.FPS,
		Format:          v.Format,
		PlaybackURL:     v.PlaybackURL,
		HLSQualities:    v.HLSQualities,
		FailureReason:   v.FailureReason,
		CreatedAt:       v.CreatedAt,
	}
}
