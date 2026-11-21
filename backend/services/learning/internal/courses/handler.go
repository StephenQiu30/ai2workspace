// Package courses provides HTTP handlers for course authoring, discovery, and internal integrations.
package courses

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/auth"
	"github.com/gin-gonic/gin"
)

// Handler handles course endpoints.
type Handler struct {
	store           Store
	serviceToken    string
	entitlements    EntitlementClient
	progressSummary ProgressSummaryProvider
}

// NewHandler initializes a courses Handler.
func NewHandler(store Store, serviceToken string, entitlements EntitlementClient, progressSummary ProgressSummaryProvider) *Handler {
	if serviceToken == "" {
		serviceToken = "identity-service-secret"
	}
	return &Handler{
		store:           store,
		serviceToken:    serviceToken,
		entitlements:    entitlements,
		progressSummary: progressSummary,
	}
}

func (h *Handler) verifyServiceToken(c *gin.Context) bool {
	token := c.GetHeader("X-Service-Token")
	return token != "" && token == h.serviceToken
}

func parsePageParams(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	rawSize := c.Query("pageSize")
	if rawSize == "" {
		rawSize = c.DefaultQuery("limit", "20")
	}
	pageSize, _ = strconv.Atoi(rawSize)
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func (h *Handler) requireCourseOwnerOrAdmin(c *gin.Context, courseID string) (*auth.Principal, *Course, bool) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return nil, nil, false
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Instructor or admin role required"})
		return nil, nil, false
	}
	course, err := h.store.GetCourse(c.Request.Context(), courseID)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
			return nil, nil, false
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to load course"})
		return nil, nil, false
	}
	if !p.IsAdmin() && course.OwnerID != p.UserID {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "You do not own this course"})
		return nil, nil, false
	}
	return p, course, true
}

func (h *Handler) requireAdmin(c *gin.Context) (*auth.Principal, bool) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return nil, false
	}
	if !p.IsAdmin() {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Admin privileges required"})
		return nil, false
	}
	return p, true
}

// CheckUserCourseAccess evaluates whether a user has current content access to a course.
// Returns (allowed, reason, error). A non-nil error indicates a downstream dependency failure (fail-closed).
func (h *Handler) CheckUserCourseAccess(ctx context.Context, userID string, roles []string, course *Course, purpose string) (bool, string, error) {
	if course == nil {
		return false, "course_not_found", nil
	}
	if course.DeliveryState == "withdrawn" {
		return false, "course_withdrawn", nil
	}
	if course.DeliveryState != "available" {
		return false, "course_unavailable", nil
	}
	if userID == "" {
		return false, "unauthenticated", nil
	}
	if course.OwnerID == userID {
		return true, "course_owner", nil
	}
	for _, r := range roles {
		if strings.EqualFold(strings.TrimSpace(r), "admin") {
			return true, "admin", nil
		}
	}
	if course.ProductionState != "approved" || course.PublishedVersion == 0 {
		return false, "course_not_published", nil
	}

	if course.IsFree {
		enrolled, err := h.store.IsUserEnrolled(ctx, userID, course.ID)
		if err != nil {
			return false, "enrollment_check_failed", err
		}
		if enrolled {
			return true, "free_enrolled", nil
		}
		return false, "not_enrolled", nil
	}

	if h.entitlements == nil {
		return false, "entitlement_required", nil
	}
	if purpose == "" {
		purpose = "playback"
	}
	allowed, reason, err := h.entitlements.CheckEntitlement(ctx, userID, course.ID, purpose)
	if err != nil {
		return false, "entitlement_service_unavailable", err
	}
	if allowed {
		if reason == "" {
			reason = "entitlement_active"
		}
		return true, reason, nil
	}
	if reason == "" {
		reason = "entitlement_required"
	}
	return false, reason, nil
}

// ListCourses godoc
// @Summary List published courses
// @Description Returns paginated courses approved for public listing with keyword search and category filtering.
// @Tags courses
// @Produce json
// @Param q query string false "Search keyword (title or description)"
// @Param category query string false "Category filter (e.g. engineering, ai, product, design, general)"
// @Param page query int false "Page number"
// @Param pageSize query int false "Page size"
// @Success 200 {object} CourseListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/learning/courses [get]
// @id learningListCourses
func (h *Handler) ListCourses(c *gin.Context) {
	page, pageSize := parsePageParams(c)
	items, total, err := h.store.ListCourses(c.Request.Context(), ListFilter{
		PublishedOnly: true,
		Query:         c.Query("q"),
		Category:      c.Query("category"),
		Page:          page,
		PageSize:      pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to list courses"})
		return
	}

	responses := make([]CourseResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toSanitizedCourseResponse(item, false))
	}

	c.JSON(http.StatusOK, CourseListResponse{
		Courses:  responses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// ListManageCourses godoc
// @Summary List courses for management
// @Description Returns paginated courses owned by the instructor, or all courses for administrators.
// @Tags courses
// @Produce json
// @Param state query string false "State filter (draft, pending_review, approved, rejected, closed, withdrawn)"
// @Param q query string false "Search keyword"
// @Param category query string false "Category filter"
// @Param page query int false "Page number"
// @Param pageSize query int false "Page size"
// @Success 200 {object} CourseListResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/learning/manage/courses [get]
// @id learningListManageCourses
func (h *Handler) ListManageCourses(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Instructor or admin role required"})
		return
	}

	page, pageSize := parsePageParams(c)
	filter := ListFilter{
		PublishedOnly: false,
		Query:         c.Query("q"),
		Category:      c.Query("category"),
		State:         c.Query("state"),
		Page:          page,
		PageSize:      pageSize,
	}
	if !p.IsAdmin() {
		filter.OwnerID = p.UserID
	}

	items, total, err := h.store.ListCourses(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to list managed courses"})
		return
	}

	responses := make([]CourseResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toSanitizedCourseResponse(item, true))
	}

	c.JSON(http.StatusOK, CourseListResponse{
		Courses:  responses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetCourse godoc
// @Summary Get course details
// @Description Retrieves details of a course by ID. Draft/withdrawn courses are hidden from unauthorized callers, and non-trial lesson content is withheld unless the caller has course access.
// @Tags courses
// @Produce json
// @Param id path string true "Course ID"
// @Success 200 {object} CourseResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/learning/courses/{id} [get]
// @id learningGetCourse
func (h *Handler) GetCourse(c *gin.Context) {
	courseID := c.Param("id")
	course, err := h.store.GetCourse(c.Request.Context(), courseID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		return
	}

	p, _ := auth.FromContext(c)
	isActiveUser := p != nil && p.IsActive()
	isOwnerOrAdmin := isActiveUser && (p.IsAdmin() || course.OwnerID == p.UserID)

	isPublished := course.ProductionState == "approved" && course.PublishedVersion > 0
	isWithdrawn := course.DeliveryState == "withdrawn"
	if (!isPublished || isWithdrawn) && !isOwnerOrAdmin {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		return
	}

	hasAccess := isOwnerOrAdmin
	if !hasAccess && isActiveUser && !isWithdrawn {
		allowed, _, checkErr := h.CheckUserCourseAccess(c.Request.Context(), p.UserID, p.Roles, course, "playback")
		if checkErr == nil && allowed {
			hasAccess = true
		}
	}

	c.JSON(http.StatusOK, toSanitizedCourseResponse(course, hasAccess))
}

// CreateCourse godoc
// @Summary Create a draft course
// @Description Initiates a new course draft (instructor or admin only).
// @Tags courses
// @Accept json
// @Produce json
// @Param body body CreateCourseRequest true "Course creation payload"
// @Success 200 {object} CourseMutationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/learning/courses [post]
// @id learningCreateCourse
func (h *Handler) CreateCourse(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Instructor or admin role required"})
		return
	}

	var req CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Title is required"})
		return
	}

	course, err := h.store.CreateCourse(
		c.Request.Context(),
		p.UserID,
		p.DisplayName(),
		strings.TrimSpace(req.Title),
		req.Description,
		req.CoverURL,
		req.Category,
		req.IsFree,
	)
	if err != nil {
		if errors.Is(err, ErrInvalidCategory) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_category", Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "create_failed", Message: "Failed to create course"})
		return
	}

	c.JSON(http.StatusOK, CourseMutationResponse{
		CourseID: course.ID,
		State:    course.ProductionState,
		Version:  course.Version,
	})
}

// UpdateCourse godoc
// @Summary Update course details
// @Description Modifies an existing course with ownership and version checking (course owner or admin).
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body UpdateCourseRequest true "Update payload"
// @Success 200 {object} CourseMutationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id} [patch]
// @id learningUpdateCourse
func (h *Handler) UpdateCourse(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Instructor or admin role required"})
		return
	}
	courseID := c.Param("id")

	var req UpdateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "expectedVersion is required"})
		return
	}

	course, err := h.store.UpdateCourse(
		c.Request.Context(),
		courseID,
		p.UserID,
		p.IsAdmin(),
		strings.TrimSpace(req.Title),
		req.Description,
		req.CoverURL,
		req.Category,
		req.IsFree,
		req.ExpectedVersion,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Course has been modified elsewhere"})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "You do not own this course"})
		case errors.Is(err, ErrCourseNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		case errors.Is(err, ErrInvalidCategory):
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_category", Message: err.Error()})
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "update_failed", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, CourseMutationResponse{
		CourseID: course.ID,
		State:    course.ProductionState,
		Version:  course.Version,
	})
}

// DeleteCourse godoc
// @Summary Delete an unpublished course
// @Description Permanently deletes an unpublished draft course with ownership and version check.
// @Tags courses
// @Produce json
// @Param id path string true "Course ID"
// @Param expectedVersion query int true "Expected version"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id} [delete]
// @id learningDeleteCourse
func (h *Handler) DeleteCourse(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Instructor or admin role required"})
		return
	}
	courseID := c.Param("id")
	expectedVersionStr := c.Query("expectedVersion")
	version, err := strconv.Atoi(expectedVersionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_version", Message: "expectedVersion query parameter must be an integer"})
		return
	}

	if err := h.store.DeleteCourse(c.Request.Context(), courseID, p.UserID, p.IsAdmin(), version); err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Version conflict"})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Permission denied"})
		case errors.Is(err, ErrCourseNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		case errors.Is(err, ErrCoursePublished):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "course_published", Message: err.Error()})
		default:
			c.JSON(http.StatusConflict, ErrorResponse{Error: "delete_failed", Message: err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// SubmitCourse godoc
// @Summary Submit course for review
// @Description Submits a draft or rejected course for administrator review and publication.
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body SubmitCourseRequest true "Submission payload"
// @Success 200 {object} CourseMutationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id}/submissions [post]
// @id learningSubmitCourse
func (h *Handler) SubmitCourse(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}
	if !p.IsInstructorOrAdmin() {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Instructor or admin role required"})
		return
	}
	courseID := c.Param("id")

	var req SubmitCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "expectedVersion is required"})
		return
	}

	course, err := h.store.SubmitCourse(c.Request.Context(), courseID, p.UserID, p.IsAdmin(), req.ExpectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Version conflict"})
		case errors.Is(err, ErrInvalidState):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "invalid_state", Message: "Only draft or rejected courses can be submitted for review"})
		case errors.Is(err, ErrPermissionDenied):
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Permission denied"})
		case errors.Is(err, ErrCourseNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "submit_failed", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, CourseMutationResponse{
		CourseID: course.ID,
		State:    course.ProductionState,
		Version:  course.Version,
	})
}

// ReviewCourse godoc
// @Summary Review course submission
// @Description Approves or rejects a pending_review course (admin only).
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body ReviewCourseRequest true "Review decision payload"
// @Success 200 {object} CourseMutationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id}/reviews [post]
// @id learningReviewCourse
func (h *Handler) ReviewCourse(c *gin.Context) {
	if _, ok := h.requireAdmin(c); !ok {
		return
	}
	courseID := c.Param("id")

	var req ReviewCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "decision and expectedVersion are required"})
		return
	}
	if req.Decision != "approved" && req.Decision != "rejected" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_decision", Message: "decision must be approved or rejected"})
		return
	}

	course, err := h.store.ReviewCourse(c.Request.Context(), courseID, req.Decision, req.Reason, req.ExpectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Version conflict"})
		case errors.Is(err, ErrInvalidState):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "invalid_state", Message: "Only courses in pending_review state can be reviewed"})
		case errors.Is(err, ErrCourseNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "review_failed", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, CourseMutationResponse{
		CourseID: course.ID,
		State:    course.ProductionState,
		Version:  course.Version,
	})
}

// CloseSales godoc
// @Summary Close course sales
// @Description Halts new purchases of a course while preserving access for enrolled students (admin only).
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body CloseSalesRequest true "Sales closure payload"
// @Success 200 {object} CourseMutationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id}/sales-closure [post]
// @id learningCloseSales
func (h *Handler) CloseSales(c *gin.Context) {
	p, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	courseID := c.Param("id")

	var req CloseSalesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "expectedVersion is required"})
		return
	}

	course, err := h.store.CloseSales(c.Request.Context(), courseID, p.UserID, req.Reason, req.ExpectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Version conflict"})
		case errors.Is(err, ErrCourseNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "operation_failed", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, CourseMutationResponse{
		CourseID: course.ID,
		State:    course.SaleState,
		Version:  course.Version,
	})
}

// WithdrawCourse godoc
// @Summary Withdraw course delivery
// @Description Withdraws a course completely from delivery for compliance reasons (admin only).
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body WithdrawCourseRequest true "Withdrawal payload"
// @Success 200 {object} CourseMutationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id}/withdrawals [post]
// @id learningWithdrawCourse
func (h *Handler) WithdrawCourse(c *gin.Context) {
	p, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	courseID := c.Param("id")

	var req WithdrawCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "expectedVersion and reason are required"})
		return
	}

	course, err := h.store.WithdrawCourse(c.Request.Context(), courseID, p.UserID, req.Reason, req.ExpectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Version conflict"})
		case errors.Is(err, ErrCourseNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "operation_failed", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, CourseMutationResponse{
		CourseID: course.ID,
		State:    course.DeliveryState,
		Version:  course.Version,
	})
}

// AddSection godoc
// @Summary Add a section to course
// @Description Adds a chapter section to the course curriculum (course owner or admin).
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body AddSectionRequest true "Section payload"
// @Success 200 {object} Section
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/learning/courses/{id}/sections [post]
// @id learningAddSection
func (h *Handler) AddSection(c *gin.Context) {
	courseID := c.Param("id")
	if _, _, ok := h.requireCourseOwnerOrAdmin(c, courseID); !ok {
		return
	}

	var req AddSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Title is required"})
		return
	}

	sec, err := h.store.AddSection(c.Request.Context(), courseID, strings.TrimSpace(req.Title), req.SortOrder)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "section_create_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, sec)
}

// UpdateSection godoc
// @Summary Update a section in course
// @Description Updates a chapter section's title or sort order (course owner or admin).
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param sectionId path string true "Section ID"
// @Param body body UpdateSectionRequest true "Section update payload"
// @Success 200 {object} Section
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/learning/courses/{id}/sections/{sectionId} [patch]
// @id learningUpdateSection
func (h *Handler) UpdateSection(c *gin.Context) {
	courseID := c.Param("id")
	sectionID := c.Param("sectionId")
	if _, _, ok := h.requireCourseOwnerOrAdmin(c, courseID); !ok {
		return
	}

	var req UpdateSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Invalid section update payload"})
		return
	}

	sec, err := h.store.UpdateSection(c.Request.Context(), courseID, sectionID, req.Title, req.SortOrder)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) || errors.Is(err, ErrSectionNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "section_not_found", Message: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "section_update_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, sec)
}

// DeleteSection godoc
// @Summary Delete a section from course
// @Description Deletes a chapter section and its lessons from the course (course owner or admin).
// @Tags courses
// @Produce json
// @Param id path string true "Course ID"
// @Param sectionId path string true "Section ID"
// @Success 204 "No Content"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/learning/courses/{id}/sections/{sectionId} [delete]
// @id learningDeleteSection
func (h *Handler) DeleteSection(c *gin.Context) {
	courseID := c.Param("id")
	sectionID := c.Param("sectionId")
	if _, _, ok := h.requireCourseOwnerOrAdmin(c, courseID); !ok {
		return
	}

	if err := h.store.DeleteSection(c.Request.Context(), courseID, sectionID); err != nil {
		if errors.Is(err, ErrCourseNotFound) || errors.Is(err, ErrSectionNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "section_not_found", Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "section_delete_failed", Message: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// AddLesson godoc
// @Summary Add a lesson to course section
// @Description Adds an article or video lesson to a specified course section (course owner or admin).
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body AddLessonRequest true "Lesson payload"
// @Success 200 {object} Lesson
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/learning/courses/{id}/lessons [post]
// @id learningAddLesson
func (h *Handler) AddLesson(c *gin.Context) {
	courseID := c.Param("id")
	if _, _, ok := h.requireCourseOwnerOrAdmin(c, courseID); !ok {
		return
	}

	var req AddLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "SectionID, Title, and Type are required"})
		return
	}

	lsn, err := h.store.AddLesson(c.Request.Context(), courseID, req.SectionID, strings.TrimSpace(req.Title), req.Type, req.IsTrial, req.Content, req.MediaVersionID, req.SortOrder)
	if err != nil {
		switch {
		case errors.Is(err, ErrCourseNotFound), errors.Is(err, ErrSectionNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "not_found", Message: err.Error()})
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "lesson_creation_failed", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, lsn)
}

// UpdateLesson godoc
// @Summary Update a lesson in course
// @Description Updates a lesson's metadata, content, trial flag, or ordering (course owner or admin).
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param lessonId path string true "Lesson ID"
// @Param body body UpdateLessonRequest true "Lesson update payload"
// @Success 200 {object} Lesson
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/learning/courses/{id}/lessons/{lessonId} [patch]
// @id learningUpdateLesson
func (h *Handler) UpdateLesson(c *gin.Context) {
	courseID := c.Param("id")
	lessonID := c.Param("lessonId")
	if _, _, ok := h.requireCourseOwnerOrAdmin(c, courseID); !ok {
		return
	}

	var req UpdateLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Invalid lesson update payload"})
		return
	}

	lsn, err := h.store.UpdateLesson(c.Request.Context(), courseID, lessonID, LessonPatch{
		SectionID:      req.SectionID,
		Title:          req.Title,
		Type:           req.Type,
		IsTrial:        req.IsTrial,
		Content:        req.Content,
		MediaVersionID: req.MediaVersionID,
		SortOrder:      req.SortOrder,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrCourseNotFound), errors.Is(err, ErrSectionNotFound), errors.Is(err, ErrLessonNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "not_found", Message: err.Error()})
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "lesson_update_failed", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, lsn)
}

// DeleteLesson godoc
// @Summary Delete a lesson from course
// @Description Deletes a lesson from a course (course owner or admin).
// @Tags courses
// @Produce json
// @Param id path string true "Course ID"
// @Param lessonId path string true "Lesson ID"
// @Success 204 "No Content"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/learning/courses/{id}/lessons/{lessonId} [delete]
// @id learningDeleteLesson
func (h *Handler) DeleteLesson(c *gin.Context) {
	courseID := c.Param("id")
	lessonID := c.Param("lessonId")
	if _, _, ok := h.requireCourseOwnerOrAdmin(c, courseID); !ok {
		return
	}

	if err := h.store.DeleteLesson(c.Request.Context(), courseID, lessonID); err != nil {
		if errors.Is(err, ErrCourseNotFound) || errors.Is(err, ErrLessonNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "lesson_not_found", Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "lesson_delete_failed", Message: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// JoinFreeCourse godoc
// @Summary Join a free course
// @Description Enrolls authenticated user in a published free course.
// @Tags courses
// @Produce json
// @Param id path string true "Course ID"
// @Success 200 {object} Enrollment
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id}/enrollments [post]
// @id learningJoinFreeCourse
func (h *Handler) JoinFreeCourse(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}

	courseID := c.Param("id")
	enr, err := h.store.EnrollUser(c.Request.Context(), p.UserID, courseID, "free")
	if err != nil {
		switch {
		case errors.Is(err, ErrCourseNotFound), errors.Is(err, ErrCourseUnpublished):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: err.Error()})
		case errors.Is(err, ErrCourseWithdrawn):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "course_withdrawn", Message: err.Error()})
		case errors.Is(err, ErrAlreadyEnrolled):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "already_enrolled", Message: err.Error()})
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "enroll_failed", Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, enr)
}

// ListMyCourses godoc
// @Summary List current user enrolled and purchased courses
// @Description Merges local free enrollments and commerce purchased entitlements with learning progress summaries.
// @Tags courses
// @Produce json
// @Success 200 {array} MyCourseResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/learning/me/courses [get]
// @id learningListMyCourses
func (h *Handler) ListMyCourses(c *gin.Context) {
	p, ok := auth.RequirePrincipal(c)
	if !ok {
		return
	}

	enrollments, err := h.store.ListUserEnrollments(c.Request.Context(), p.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to load enrollments"})
		return
	}

	courseTypes := make(map[string]string, len(enrollments))
	var courseOrder []string
	for _, e := range enrollments {
		if _, exists := courseTypes[e.CourseID]; !exists {
			courseOrder = append(courseOrder, e.CourseID)
		}
		courseTypes[e.CourseID] = e.Type
	}

	if h.entitlements != nil {
		ents, err := h.entitlements.ListUserEntitlements(c.Request.Context(), p.UserID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "entitlement_service_unavailable", Message: "Failed to load purchased entitlements"})
			return
		}
		for _, ent := range ents {
			status := strings.ToLower(strings.TrimSpace(ent.Status))
			if status != "" && status != "active" {
				continue
			}
			if _, exists := courseTypes[ent.CourseID]; !exists {
				courseOrder = append(courseOrder, ent.CourseID)
			}
			courseTypes[ent.CourseID] = "purchased"
		}
	}

	responses := make([]MyCourseResponse, 0, len(courseOrder))
	for _, cid := range courseOrder {
		course, err := h.store.GetCourse(c.Request.Context(), cid)
		if err != nil {
			continue
		}
		hasContentAccess := course.DeliveryState != "withdrawn"
		summary := CourseProgressSummary{TotalLessonCount: course.LessonCount}
		if h.progressSummary != nil {
			if s, err := h.progressSummary(c.Request.Context(), p.UserID, course); err == nil {
				summary = s
			}
		}
		responses = append(responses, MyCourseResponse{
			CourseResponse:  toSanitizedCourseResponse(course, hasContentAccess),
			EnrollmentType:  courseTypes[cid],
			ProgressSummary: summary,
		})
	}

	sort.SliceStable(responses, func(i, j int) bool {
		ti := responses[i].ProgressSummary.LastStudiedAt
		tj := responses[j].ProgressSummary.LastStudiedAt
		if ti != nil && tj != nil && !ti.Equal(*tj) {
			return ti.After(*tj)
		}
		if ti != nil && tj == nil {
			return true
		}
		if ti == nil && tj != nil {
			return false
		}
		if !responses[i].UpdatedAt.Equal(responses[j].UpdatedAt) {
			return responses[i].UpdatedAt.After(responses[j].UpdatedAt)
		}
		return responses[i].ID > responses[j].ID
	})

	c.JSON(http.StatusOK, responses)
}

// GetSaleSnapshot godoc
// @Summary Internal sale snapshot query
// @Description Provides course sales snapshot for commerce checkout without querying entitlement.
// @Tags internal
// @Produce json
// @Param courseId path string true "Course ID"
// @Param X-Service-Token header string true "Service authentication token"
// @Success 200 {object} SaleSnapshotResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /internal/learning/courses/{courseId}/sale-snapshot [get]
// @id learningGetSaleSnapshot
func (h *Handler) GetSaleSnapshot(c *gin.Context) {
	if !h.verifyServiceToken(c) {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized_service", Message: "Valid service token required"})
		return
	}

	courseID := c.Param("courseId")
	course, err := h.store.GetCourse(c.Request.Context(), courseID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		return
	}

	deliverable := course.ProductionState == "approved" && course.PublishedVersion > 0 && course.DeliveryState == "available"
	c.JSON(http.StatusOK, SaleSnapshotResponse{
		CourseID:         course.ID,
		PublishedVersion: course.PublishedVersion,
		Title:            course.Title,
		IsFree:           course.IsFree,
		SaleState:        course.SaleState,
		Deliverable:      deliverable,
	})
}

// CheckCourseAccess godoc
// @Summary Internal course access check
// @Description Validates whether a user can access a course for playback, discussion, or knowledge.
// @Tags internal
// @Accept json
// @Produce json
// @Param X-Service-Token header string true "Service authentication token"
// @Param body body CheckCourseAccessRequest true "Access check payload"
// @Success 200 {object} CheckCourseAccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /internal/learning/course-access/check [post]
// @id learningCheckCourseAccess
func (h *Handler) CheckCourseAccess(c *gin.Context) {
	if !h.verifyServiceToken(c) {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized_service", Message: "Valid service token required"})
		return
	}

	var req CheckCourseAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "userId, courseId, and purpose required"})
		return
	}

	course, err := h.store.GetCourse(c.Request.Context(), req.CourseID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		return
	}

	allowed, reason, checkErr := h.CheckUserCourseAccess(c.Request.Context(), req.UserID, req.Roles, course, req.Purpose)
	if checkErr != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "entitlement_service_unavailable", Message: "Failed to verify entitlement"})
		return
	}

	c.JSON(http.StatusOK, CheckCourseAccessResponse{
		Allowed:        allowed,
		Reason:         reason,
		AccessVersion:  course.Version,
		ContentVersion: course.PublishedVersion,
	})
}

// CheckCourseManagement godoc
// @Summary Internal management permission check
// @Description Verifies if an actor possesses management rights over a course resource.
// @Tags internal
// @Accept json
// @Produce json
// @Param X-Service-Token header string true "Service authentication token"
// @Param body body CheckCourseManagementRequest true "Management check payload"
// @Success 200 {object} CheckCourseManagementResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /internal/learning/course-management/check [post]
// @id learningCheckCourseManagement
func (h *Handler) CheckCourseManagement(c *gin.Context) {
	if !h.verifyServiceToken(c) {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized_service", Message: "Valid service token required"})
		return
	}

	var req CheckCourseManagementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "actorId, courseId, and purpose required"})
		return
	}

	course, err := h.store.GetCourse(c.Request.Context(), req.CourseID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		return
	}

	allowed := course.OwnerID == req.ActorID
	if !allowed {
		for _, r := range req.ActorRoles {
			if strings.EqualFold(strings.TrimSpace(r), "admin") {
				allowed = true
				break
			}
		}
	}
	c.JSON(http.StatusOK, CheckCourseManagementResponse{
		Allowed:         allowed,
		ResourceVersion: course.Version,
	})
}

// GetDeliveryEvidence godoc
// @Summary Internal delivery evidence query
// @Description Returns earliest deliverable timestamp for commerce refund window calculation.
// @Tags internal
// @Produce json
// @Param courseId path string true "Course ID"
// @Param notBefore query string false "Grant activation timestamp RFC3339"
// @Param X-Service-Token header string true "Service authentication token"
// @Success 200 {object} DeliveryEvidenceResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /internal/learning/courses/{courseId}/delivery-evidence [get]
// @id learningGetDeliveryEvidence
func (h *Handler) GetDeliveryEvidence(c *gin.Context) {
	if !h.verifyServiceToken(c) {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized_service", Message: "Valid service token required"})
		return
	}

	courseID := c.Param("courseId")
	notBeforeStr := c.Query("notBefore")
	var notBefore *time.Time
	if notBeforeStr != "" {
		if t, err := time.Parse(time.RFC3339, notBeforeStr); err == nil {
			notBefore = &t
		}
	}

	firstAt, evVer, cntVer, err := h.store.GetDeliveryEvidence(c.Request.Context(), courseID, notBefore)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
		return
	}

	c.JSON(http.StatusOK, DeliveryEvidenceResponse{
		CourseID:           courseID,
		FirstDeliverableAt: firstAt,
		EvidenceVersion:    evVer,
		ContentVersion:     cntVer,
	})
}

func toSanitizedCourseResponse(c *Course, includeProtectedContent bool) CourseResponse {
	sections := make([]Section, len(c.Sections))
	lessonCount := 0
	for i, sec := range c.Sections {
		secCopy := sec
		lessons := make([]Lesson, len(sec.Lessons))
		for j, l := range sec.Lessons {
			lCopy := l
			if !includeProtectedContent && !lCopy.IsTrial {
				lCopy.Content = ""
			}
			lessons[j] = lCopy
		}
		secCopy.Lessons = lessons
		sections[i] = secCopy
		lessonCount += len(lessons)
	}
	if c.LessonCount > lessonCount {
		lessonCount = c.LessonCount
	}
	return CourseResponse{
		ID:                 c.ID,
		OwnerID:            c.OwnerID,
		AuthorNickname:     c.AuthorNickname,
		Title:              c.Title,
		Description:        c.Description,
		CoverURL:           c.CoverURL,
		Category:           c.Category,
		IsFree:             c.IsFree,
		ProductionState:    c.ProductionState,
		SaleState:          c.SaleState,
		DeliveryState:      c.DeliveryState,
		ReviewReason:       c.ReviewReason,
		LessonCount:        lessonCount,
		PublishedVersion:   c.PublishedVersion,
		Version:            c.Version,
		FirstDeliverableAt: c.FirstDeliverableAt,
		Sections:           sections,
		CreatedAt:          c.CreatedAt,
		UpdatedAt:          c.UpdatedAt,
	}
}
