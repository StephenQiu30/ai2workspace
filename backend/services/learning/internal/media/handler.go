package media

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/auth"
	"github.com/gin-gonic/gin"
)

// LessonContext holds authoritative course and lesson metadata from courses.Store.
type LessonContext struct {
	LessonID         string
	CourseID         string
	OwnerID          string
	IsFree           bool
	IsTrial          bool
	ProductionState  string
	DeliveryState    string
	PublishedVersion int
	MediaVersionID   string
}

// LessonResolver resolves authoritative lesson and course state by lessonID.
type LessonResolver func(ctx context.Context, lessonID string) (*LessonContext, error)

// CourseAccessChecker verifies whether an authenticated principal holds current playback access to a course.
type CourseAccessChecker func(ctx context.Context, p *auth.Principal, courseID string) (bool, string, error)

// LessonMediaSyncer updates the lesson's mediaVersionId in courses.Store upon publication.
type LessonMediaSyncer func(ctx context.Context, lessonID, mediaVersionID string) error

// Handler handles media-related HTTP endpoints.
type Handler struct {
	store          Store
	lessonResolver LessonResolver
	accessChecker  CourseAccessChecker
	mediaSyncer    LessonMediaSyncer
	signingSecret  string
}

// NewHandler creates a new media Handler.
func NewHandler(store Store, resolver LessonResolver, checker CourseAccessChecker, syncer LessonMediaSyncer, signingSecret string) *Handler {
	if signingSecret == "" {
		signingSecret = "identity-service-secret"
	}
	return &Handler{
		store:          store,
		lessonResolver: resolver,
		accessChecker:  checker,
		mediaSyncer:    syncer,
		signingSecret:  signingSecret,
	}
}

// CreateUpload initiates a controlled media upload session.
// @ID learningCreateUpload
// @Summary Initiate direct video upload
// @Description Creates a short-lived direct upload slot enforcing MP4, <=2GiB, max 2 concurrent uploads (course owner or admin).
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Lesson ID"
// @Param request body CreateUploadRequest true "Upload metadata"
// @Success 201 {object} UploadSessionResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 413 {object} map[string]string
// @Failure 429 {object} map[string]string
// @Router /api/learning/lessons/{id}/uploads [post]
func (h *Handler) CreateUpload(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "Instructor or admin role required"})
		return
	}

	lessonID := c.Param("id")
	var req CreateUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	if h.lessonResolver != nil {
		lctx, err := h.lessonResolver(c.Request.Context(), lessonID)
		if err != nil || lctx == nil || lctx.CourseID != req.CourseID {
			c.JSON(http.StatusNotFound, gin.H{"error": "lesson_not_found", "message": "Lesson not found in specified course"})
			return
		}
		if !p.IsAdmin() && lctx.OwnerID != p.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You do not own this course"})
			return
		}
	}

	session, err := h.store.CreateUpload(c.Request.Context(), p.UserID, req.CourseID, lessonID, req.FileName, req.DeclaredBytes)
	if err != nil {
		switch {
		case errors.Is(err, ErrFileTooLarge):
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file_too_large", "message": err.Error()})
		case errors.Is(err, ErrMaxConcurrentUploads):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "max_concurrent_uploads", "message": err.Error()})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_upload", "message": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, sessionToResponse(session))
}

// CompleteUpload verifies uploaded media and creates a ready/failed media version.
// @ID learningCompleteUpload
// @Summary Complete media upload and trigger validation/packaging
// @Description Validates MP4, H.264, duration <=120m, long side <=1920, short side <=1080, fps <=30 (session owner or admin).
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Upload ID"
// @Param request body CompleteUploadRequest true "Probed media data"
// @Success 200 {object} MediaVersionResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 410 {object} map[string]string
// @Failure 422 {object} MediaVersionResponse
// @Router /api/learning/uploads/{id}/completion [post]
func (h *Handler) CompleteUpload(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "Instructor or admin role required"})
		return
	}

	uploadID := c.Param("id")
	var req CompleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
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

	version, err := h.store.CompleteUpload(c.Request.Context(), uploadID, p.UserID, p.IsAdmin(), req.IdempotencyKey, meta)
	if err != nil {
		switch {
		case errors.Is(err, ErrUploadNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "upload_not_found", "message": err.Error()})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrUploadExpired), errors.Is(err, ErrUploadCancelled):
			c.JSON(http.StatusGone, gin.H{"error": "upload_unavailable", "message": err.Error()})
		case errors.Is(err, ErrIdempotencyConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "idempotency_conflict", "message": err.Error()})
		default:
			if version != nil {
				c.JSON(http.StatusUnprocessableEntity, versionToResponse(version))
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_failed", "message": err.Error()})
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
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/learning/uploads/{id} [get]
func (h *Handler) GetUpload(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "Instructor or admin role required"})
		return
	}

	uploadID := c.Param("id")
	session, err := h.store.GetUpload(c.Request.Context(), uploadID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "upload_not_found", "message": err.Error()})
		return
	}
	if !p.IsAdmin() && session.InstructorID != p.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You do not own this upload session"})
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
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/learning/uploads/{id} [delete]
func (h *Handler) CancelUpload(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "Instructor or admin role required"})
		return
	}

	uploadID := c.Param("id")
	if err := h.store.CancelUpload(c.Request.Context(), uploadID, p.UserID, p.IsAdmin()); err != nil {
		switch {
		case errors.Is(err, ErrUploadNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "upload_not_found", "message": err.Error()})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrUploadCompleted):
			c.JSON(http.StatusConflict, gin.H{"error": "upload_completed", "message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "cancel_failed", "message": err.Error()})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// PublishLessonMedia binds a ready media version to a lesson.
// @ID learningPublishLessonMedia
// @Summary Publish ready media to lesson
// @Description Explicitly switches the lesson's published media version and updates the lesson record (course owner or admin).
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Lesson ID"
// @Param request body PublishMediaRequest true "Media Version"
// @Success 200 {object} PublishMediaResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/learning/lessons/{id}/media-publication [post]
func (h *Handler) PublishLessonMedia(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "Instructor or admin role required"})
		return
	}

	lessonID := c.Param("id")
	var req PublishMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	if h.lessonResolver != nil {
		lctx, err := h.lessonResolver(c.Request.Context(), lessonID)
		if err != nil || lctx == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "lesson_not_found", "message": "Lesson not found"})
			return
		}
		if !p.IsAdmin() && lctx.OwnerID != p.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You do not own this course"})
			return
		}
	}

	if err := h.store.PublishLessonMedia(c.Request.Context(), lessonID, req.MediaVersionID, p.UserID, p.IsAdmin()); err != nil {
		switch {
		case errors.Is(err, ErrMediaVersionNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "media_version_not_found", "message": err.Error()})
		case errors.Is(err, ErrMediaNotReady), errors.Is(err, ErrLessonMismatch):
			c.JSON(http.StatusConflict, gin.H{"error": "media_publish_conflict", "message": err.Error()})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "publish_failed", "message": err.Error()})
		}
		return
	}

	if h.mediaSyncer != nil {
		if err := h.mediaSyncer(c.Request.Context(), lessonID, req.MediaVersionID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "lesson_sync_failed", "message": "Failed to update lesson media reference"})
			return
		}
	}

	c.JSON(http.StatusOK, PublishMediaResponse{
		Status:         "published",
		LessonID:       lessonID,
		MediaVersionID: req.MediaVersionID,
	})
}

// RetryMediaJob retries a failed asynchronous transcoding job.
// @ID learningRetryMediaJob
// @Summary Retry failed media processing job
// @Tags Media
// @Produce json
// @Param id path string true "Job ID"
// @Success 200 {object} MediaJobResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/learning/media-jobs/{id}/retries [post]
func (h *Handler) RetryMediaJob(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "Instructor or admin role required"})
		return
	}

	jobID := c.Param("id")
	job, err := h.store.RetryMediaJob(c.Request.Context(), jobID, p.UserID, p.IsAdmin())
	if err != nil {
		switch {
		case errors.Is(err, ErrMediaJobNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "media_job_not_found", "message": err.Error()})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrJobNotRetryable):
			c.JSON(http.StatusConflict, gin.H{"error": "job_not_retryable", "message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "retry_failed", "message": err.Error()})
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

// AuthorizePlayback issues a short-lived HMAC-SHA256 signed playback token for authorized users.
// @ID learningAuthorizePlayback
// @Summary Authorize HLS playback
// @Description Verifies authoritative lesson/course state, server-side trial eligibility, and user entitlement before issuing a short-lived signed HLS playback token.
// @Tags Media
// @Accept json
// @Produce json
// @Param id path string true "Lesson ID"
// @Param request body AuthorizePlaybackRequest true "Playback Request"
// @Success 200 {object} PlaybackAuthorizationResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /api/learning/lessons/{id}/playback-authorizations [post]
func (h *Handler) AuthorizePlayback(c *gin.Context) {
	lessonID := c.Param("id")
	var req AuthorizePlaybackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	if h.lessonResolver == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration_error", "message": "Lesson resolver not configured"})
		return
	}

	lctx, err := h.lessonResolver(c.Request.Context(), lessonID)
	if err != nil || lctx == nil || lctx.CourseID != req.CourseID {
		c.JSON(http.StatusNotFound, gin.H{"error": "lesson_not_found", "message": "Lesson not found"})
		return
	}

	p, authErr := auth.FromContext(c)
	if errors.Is(authErr, auth.ErrAccountSuspended) || (p != nil && !p.IsActive()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "account_suspended", "message": "Account has been suspended"})
		return
	}

	if lctx.DeliveryState == "withdrawn" {
		c.JSON(http.StatusForbidden, gin.H{"error": "course_withdrawn", "message": "Course has been withdrawn from delivery"})
		return
	}
	if lctx.DeliveryState != "available" {
		c.JSON(http.StatusForbidden, gin.H{"error": "course_unavailable", "message": "Course content is temporarily unavailable"})
		return
	}

	isOwnerOrAdmin := p != nil && p.IsActive() && (p.IsAdmin() || lctx.OwnerID == p.UserID)
	isPublished := lctx.ProductionState == "approved" && lctx.PublishedVersion > 0
	if !isPublished && !isOwnerOrAdmin {
		c.JSON(http.StatusNotFound, gin.H{"error": "course_not_found", "message": "Course is not published"})
		return
	}

	// Authoritative access check: NEVER trust client req.Purpose == "trial"; use server-side lctx.IsTrial.
	tokenPurpose := "playback"
	tokenUserID := "guest"
	if p != nil && p.UserID != "" {
		tokenUserID = p.UserID
	}

	if lctx.IsTrial {
		tokenPurpose = "trial"
	} else {
		if errors.Is(authErr, auth.ErrAuthUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth_unavailable", "message": "Authentication service temporarily unavailable"})
			return
		}
		if p == nil || p.UserID == "" {
			if req.Purpose == "trial" {
				c.JSON(http.StatusForbidden, gin.H{"error": "trial_not_allowed", "message": "This lesson is not available for trial playback"})
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Authentication required for non-trial playback"})
			return
		}
		if !isOwnerOrAdmin {
			if h.accessChecker == nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "Access denied: playback not entitled"})
				return
			}
			allowed, reason, checkErr := h.accessChecker(c.Request.Context(), p, lctx.CourseID)
			if checkErr != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "entitlement_service_unavailable", "message": "Failed to verify course entitlement"})
				return
			}
			if !allowed {
				if reason == "" {
					reason = "access_denied"
				}
				c.JSON(http.StatusForbidden, gin.H{"error": reason, "message": "Access denied: playback not entitled"})
				return
			}
		}
	}

	// Fetch published media version for this lesson.
	mediaVer, err := h.store.GetLessonMediaVersion(c.Request.Context(), lessonID)
	if err != nil && lctx.MediaVersionID != "" {
		mediaVer, err = h.store.GetMediaVersion(c.Request.Context(), lctx.MediaVersionID)
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "media_not_found", "message": "Lesson media version not found or not published"})
		return
	}
	if mediaVer.Status != MediaReady {
		c.JSON(http.StatusConflict, gin.H{"error": "media_not_ready", "message": "Media is not ready for playback"})
		return
	}

	expiresAt := time.Now().UTC().Add(PlaybackTokenTTL)
	token, err := SignPlaybackToken(PlaybackTokenClaims{
		MediaVersionID: mediaVer.ID,
		CourseID:       lctx.CourseID,
		LessonID:       lessonID,
		UserID:         tokenUserID,
		Purpose:        tokenPurpose,
		ExpiresAt:      expiresAt.Unix(),
	}, h.signingSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token_signing_failed", "message": "Failed to issue playback token"})
		return
	}

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
