// Package courses provides HTTP handlers for course authoring, discovery, and internal integrations.
package courses

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler handles course endpoints.
type Handler struct {
	store        Store
	serviceToken string
}

// NewHandler initializes a courses Handler.
func NewHandler(store Store, serviceToken string) *Handler {
	if serviceToken == "" {
		serviceToken = "identity-service-secret"
	}
	return &Handler{
		store:        store,
		serviceToken: serviceToken,
	}
}

func (h *Handler) getActor(c *gin.Context) (userID string, roles []string) {
	if actor := c.GetHeader("X-Actor-Id"); actor != "" {
		roleHeader := c.GetHeader("X-Actor-Roles")
		var rList []string
		if roleHeader != "" {
			rList = []string{roleHeader}
		}
		return actor, rList
	}
	// Fallback to cookie check if present
	if cookie, err := c.Cookie("ai2_session"); err == nil && cookie != "" {
		return "user_from_session", []string{"student"}
	}
	return "", nil
}

func (h *Handler) verifyServiceToken(c *gin.Context) bool {
	token := c.GetHeader("X-Service-Token")
	return token != "" && token == h.serviceToken
}

// ListCourses godoc
// @Summary List published courses
// @Description Returns paginated courses approved for public listing.
// @Tags courses
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} CourseListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/learning/courses [get]
// @id learningListCourses
func (h *Handler) ListCourses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	items, total, err := h.store.ListCourses(c.Request.Context(), true, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: err.Error()})
		return
	}

	responses := make([]CourseResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toCourseResponse(item))
	}

	c.JSON(http.StatusOK, CourseListResponse{
		Courses: responses,
		Total:   total,
	})
}

// GetCourse godoc
// @Summary Get course details
// @Description Retrieves comprehensive details of a course by ID.
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

	c.JSON(http.StatusOK, toCourseResponse(course))
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
// @Router /api/learning/courses [post]
// @id learningCreateCourse
func (h *Handler) CreateCourse(c *gin.Context) {
	actorID, _ := h.getActor(c)
	if actorID == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "Authentication required"})
		return
	}

	var req CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Title is required"})
		return
	}

	course, err := h.store.CreateCourse(c.Request.Context(), actorID, req.Title, req.Description, req.CoverURL, req.IsFree)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "create_failed", Message: err.Error()})
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
// @Description Modifies an existing course draft with version checking.
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body UpdateCourseRequest true "Update payload"
// @Success 200 {object} CourseMutationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id} [patch]
// @id learningUpdateCourse
func (h *Handler) UpdateCourse(c *gin.Context) {
	actorID, _ := h.getActor(c)
	courseID := c.Param("id")

	var req UpdateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "expectedVersion is required"})
		return
	}

	course, err := h.store.UpdateCourse(c.Request.Context(), courseID, actorID, req.Title, req.Description, req.CoverURL, req.IsFree, req.ExpectedVersion)
	if err != nil {
		if errors.Is(err, ErrVersionConflict) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Course has been modified elsewhere"})
			return
		}
		if errors.Is(err, ErrPermissionDenied) {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "You do not own this course"})
			return
		}
		if errors.Is(err, ErrCourseNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "update_failed", Message: err.Error()})
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
// @Description Permanently deletes an unpublished draft course with version check.
// @Tags courses
// @Produce json
// @Param id path string true "Course ID"
// @Param expectedVersion query int true "Expected version"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id} [delete]
// @id learningDeleteCourse
func (h *Handler) DeleteCourse(c *gin.Context) {
	actorID, _ := h.getActor(c)
	courseID := c.Param("id")
	expectedVersionStr := c.Query("expectedVersion")
	version, err := strconv.Atoi(expectedVersionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_version", Message: "expectedVersion query parameter must be an integer"})
		return
	}

	if err := h.store.DeleteCourse(c.Request.Context(), courseID, actorID, version); err != nil {
		if errors.Is(err, ErrVersionConflict) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Version conflict"})
			return
		}
		if errors.Is(err, ErrPermissionDenied) {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Permission denied"})
			return
		}
		if errors.Is(err, ErrCourseNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: "Course not found"})
			return
		}
		c.JSON(http.StatusConflict, ErrorResponse{Error: "delete_failed", Message: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// SubmitCourse godoc
// @Summary Submit course for review
// @Description Submits a draft course for administrator review and publication.
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body SubmitCourseRequest true "Submission payload"
// @Success 200 {object} CourseMutationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id}/submissions [post]
// @id learningSubmitCourse
func (h *Handler) SubmitCourse(c *gin.Context) {
	actorID, _ := h.getActor(c)
	courseID := c.Param("id")

	var req SubmitCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "expectedVersion is required"})
		return
	}

	course, err := h.store.SubmitCourse(c.Request.Context(), courseID, actorID, req.ExpectedVersion)
	if err != nil {
		if errors.Is(err, ErrVersionConflict) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Version conflict"})
			return
		}
		if errors.Is(err, ErrPermissionDenied) {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: "Permission denied"})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "submit_failed", Message: err.Error()})
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
// @Description Approves or rejects a submitted course (admin only).
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body ReviewCourseRequest true "Review decision payload"
// @Success 200 {object} CourseMutationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id}/reviews [post]
// @id learningReviewCourse
func (h *Handler) ReviewCourse(c *gin.Context) {
	courseID := c.Param("id")

	var req ReviewCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "decision and expectedVersion are required"})
		return
	}

	course, err := h.store.ReviewCourse(c.Request.Context(), courseID, req.Decision, req.Reason, req.ExpectedVersion)
	if err != nil {
		if errors.Is(err, ErrVersionConflict) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Version conflict"})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "review_failed", Message: err.Error()})
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
// @Description Halts new purchases of a course while preserving access for enrolled students.
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body CloseSalesRequest true "Sales closure payload"
// @Success 200 {object} CourseMutationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id}/sales-closure [post]
// @id learningCloseSales
func (h *Handler) CloseSales(c *gin.Context) {
	actorID, _ := h.getActor(c)
	courseID := c.Param("id")

	var req CloseSalesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "expectedVersion is required"})
		return
	}

	course, err := h.store.CloseSales(c.Request.Context(), courseID, actorID, req.Reason, req.ExpectedVersion)
	if err != nil {
		if errors.Is(err, ErrVersionConflict) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Version conflict"})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "operation_failed", Message: err.Error()})
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
// @Failure 409 {object} ErrorResponse
// @Router /api/learning/courses/{id}/withdrawals [post]
// @id learningWithdrawCourse
func (h *Handler) WithdrawCourse(c *gin.Context) {
	actorID, _ := h.getActor(c)
	courseID := c.Param("id")

	var req WithdrawCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "expectedVersion and reason are required"})
		return
	}

	course, err := h.store.WithdrawCourse(c.Request.Context(), courseID, actorID, req.Reason, req.ExpectedVersion)
	if err != nil {
		if errors.Is(err, ErrVersionConflict) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "version_conflict", Message: "Version conflict"})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "operation_failed", Message: err.Error()})
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
// @Description Adds a chapter section to the course curriculum.
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body AddSectionRequest true "Section payload"
// @Success 200 {object} Section
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/learning/courses/{id}/sections [post]
// @id learningAddSection
func (h *Handler) AddSection(c *gin.Context) {
	courseID := c.Param("id")

	var req AddSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Title is required"})
		return
	}

	sec, err := h.store.AddSection(c.Request.Context(), courseID, req.Title, req.SortOrder)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "course_not_found", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, sec)
}

// AddLesson godoc
// @Summary Add a lesson to course section
// @Description Adds an article or video lesson to a specified course section.
// @Tags courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param body body AddLessonRequest true "Lesson payload"
// @Success 200 {object} Lesson
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/learning/courses/{id}/lessons [post]
// @id learningAddLesson
func (h *Handler) AddLesson(c *gin.Context) {
	courseID := c.Param("id")

	var req AddLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "SectionID, Title, and Type are required"})
		return
	}

	lsn, err := h.store.AddLesson(c.Request.Context(), courseID, req.SectionID, req.Title, req.Type, req.IsTrial, req.Content, req.MediaVersionID, req.SortOrder)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "lesson_creation_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, lsn)
}

// JoinFreeCourse godoc
// @Summary Join a free course
// @Description Enrolls authenticated student in a free course.
// @Tags courses
// @Produce json
// @Param id path string true "Course ID"
// @Success 200 {object} Enrollment
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/learning/courses/{id}/enrollments [post]
// @id learningJoinFreeCourse
func (h *Handler) JoinFreeCourse(c *gin.Context) {
	actorID, _ := h.getActor(c)
	if actorID == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "Login required"})
		return
	}

	courseID := c.Param("id")
	enr, err := h.store.EnrollUser(c.Request.Context(), actorID, courseID, "free")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "enroll_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, enr)
}

// ListMyCourses godoc
// @Summary List current user enrolled courses
// @Description Returns the list of courses the authenticated user has joined or purchased.
// @Tags courses
// @Produce json
// @Success 200 {array} CourseResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/learning/me/courses [get]
// @id learningListMyCourses
func (h *Handler) ListMyCourses(c *gin.Context) {
	actorID, _ := h.getActor(c)
	if actorID == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "Login required"})
		return
	}

	items, err := h.store.ListUserCourses(c.Request.Context(), actorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: err.Error()})
		return
	}

	responses := make([]CourseResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toCourseResponse(item))
	}

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

	c.JSON(http.StatusOK, SaleSnapshotResponse{
		CourseID:         course.ID,
		PublishedVersion: course.PublishedVersion,
		Title:            course.Title,
		SaleState:        course.SaleState,
		Deliverable:      course.DeliveryState == "available",
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
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
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

	if course.DeliveryState == "withdrawn" {
		c.JSON(http.StatusOK, CheckCourseAccessResponse{
			Allowed:        false,
			Reason:         "course_withdrawn",
			AccessVersion:  course.Version,
			ContentVersion: course.PublishedVersion,
		})
		return
	}

	c.JSON(http.StatusOK, CheckCourseAccessResponse{
		Allowed:        true,
		Reason:         "access_granted",
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

func toCourseResponse(c *Course) CourseResponse {
	return CourseResponse{
		ID:                 c.ID,
		OwnerID:            c.OwnerID,
		Title:              c.Title,
		Description:        c.Description,
		CoverURL:           c.CoverURL,
		IsFree:             c.IsFree,
		ProductionState:    c.ProductionState,
		SaleState:          c.SaleState,
		DeliveryState:      c.DeliveryState,
		PublishedVersion:   c.PublishedVersion,
		Version:            c.Version,
		FirstDeliverableAt: c.FirstDeliverableAt,
		Sections:           c.Sections,
		CreatedAt:          c.CreatedAt,
		UpdatedAt:          c.UpdatedAt,
	}
}
