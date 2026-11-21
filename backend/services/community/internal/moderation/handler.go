package moderation

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/notification"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/posts"
	"github.com/gin-gonic/gin"
)

// CreateReportRequest defines the payload to flag violation content.
type CreateReportRequest struct {
	TargetType string `json:"targetType" binding:"required" example:"post"`
	TargetID   string `json:"targetId" binding:"required" example:"post_12345"`
	Reason     string `json:"reason" binding:"required" example:"广告垃圾信息与违规内容"`
}

// ModerateReportRequest defines the admin decision payload.
type ModerateReportRequest struct {
	Decision string `json:"decision" binding:"required" example:"approved"`
	Reason   string `json:"reason" example:"核实属违规推广，予以隐藏"`
}

// ReportListResponse represents a paginated report list for administrators.
type ReportListResponse struct {
	Items    []*Report `json:"items"`
	Total    int       `json:"total" example:"10"`
	Page     int       `json:"page" example:"1"`
	PageSize int       `json:"pageSize" example:"20"`
}

// Handler manages moderation HTTP endpoints.
type Handler struct {
	store         Store
	postStore     posts.Store
	notifStore    notification.Store
	resolver      auth.SessionResolver
	courseChecker auth.CourseAccessChecker
}

// NewHandler creates a moderation Handler wired with auth, posts, notifications, and course access verification.
func NewHandler(
	store Store,
	postStore posts.Store,
	notifStore notification.Store,
	resolver auth.SessionResolver,
	courseChecker auth.CourseAccessChecker,
) *Handler {
	return &Handler{
		store:         store,
		postStore:     postStore,
		notifStore:    notifStore,
		resolver:      resolver,
		courseChecker: courseChecker,
	}
}

func (h *Handler) checkCourseAccess(ctx context.Context, courseID, userID string) (bool, error) {
	if h.courseChecker == nil {
		return false, nil
	}
	return h.courseChecker(ctx, courseID, userID)
}

// CreateReport flags a visible post or comment for moderation review.
// @ID communityCreateReport
// @Summary Report post or comment
// @Tags Community
// @Accept json
// @Produce json
// @Param request body CreateReportRequest true "Report payload"
// @Success 201 {object} Report
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Failure 410 {object} auth.ErrorResponse
// @Router /api/community/reports [post]
func (h *Handler) CreateReport(c *gin.Context) {
	actor, ok := auth.RequireVerified(c, h.resolver)
	if !ok {
		return
	}

	var req CreateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	targetType := strings.TrimSpace(req.TargetType)
	targetID := strings.TrimSpace(req.TargetID)
	reason := strings.TrimSpace(req.Reason)
	if targetType != "post" && targetType != "comment" {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_target_type", Message: ErrInvalidTargetType.Error()})
		return
	}
	if reason == "" {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: "reason must not be empty"})
		return
	}
	if h.postStore == nil {
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "internal_error", Message: "post store unavailable"})
		return
	}

	var targetPost *posts.Post
	if targetType == "post" {
		p, err := h.postStore.GetPost(c.Request.Context(), targetID)
		if err != nil {
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "target_not_found", Message: "target post not found"})
			return
		}
		if p.Status == posts.StatusDeleted || p.Status == posts.StatusHidden {
			c.JSON(http.StatusGone, auth.ErrorResponse{Error: "target_unavailable", Message: "target post is already unavailable"})
			return
		}
		if p.Status != posts.StatusPublished {
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "target_not_found", Message: "target post not found"})
			return
		}
		targetPost = p
	} else {
		cmt, err := h.postStore.GetComment(c.Request.Context(), targetID)
		if err != nil {
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "target_not_found", Message: "target comment not found"})
			return
		}
		if cmt.Status != posts.StatusPublished {
			c.JSON(http.StatusGone, auth.ErrorResponse{Error: "target_unavailable", Message: "target comment is already unavailable"})
			return
		}
		p, err := h.postStore.GetPost(c.Request.Context(), cmt.PostID)
		if err != nil || p.Status != posts.StatusPublished {
			c.JSON(http.StatusGone, auth.ErrorResponse{Error: "target_unavailable", Message: "parent post is unavailable"})
			return
		}
		targetPost = p
	}

	if targetPost.Scope == posts.ScopeCourse {
		allowed, err := h.checkCourseAccess(c.Request.Context(), targetPost.CourseID, actor.UserID)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, auth.ErrorResponse{Error: "course_access_unavailable", Message: "course access check unavailable"})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, auth.ErrorResponse{Error: "course_access_denied", Message: "cannot report content outside accessible courses"})
			return
		}
	}

	report, err := h.store.CreateReport(c.Request.Context(), actor.UserID, targetType, targetID, reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "report_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, report)
}

// ModerateReport executes an admin decision ("approved" or "rejected") on a pending report.
// Approving hides the reported content first and fails if hiding fails; upon completion both reporter and content author are notified safely.
// @ID communityModerateReport
// @Summary Review report and moderate content (Admin)
// @Description Strictly restricted to admin role. Approving a report hides target content without leaking hidden body in notifications.
// @Tags Community
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body ModerateReportRequest true "Moderation decision"
// @Success 200 {object} Report
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Failure 404 {object} auth.ErrorResponse
// @Failure 409 {object} auth.ErrorResponse
// @Router /api/community/reports/{id}/review [post]
func (h *Handler) ModerateReport(c *gin.Context) {
	admin, ok := auth.RequireAdmin(c, h.resolver)
	if !ok {
		return
	}

	reportID := c.Param("id")
	var req ModerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	decision := ReportStatus(strings.TrimSpace(req.Decision))
	if decision != StatusApproved && decision != StatusRejected {
		c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_decision", Message: ErrInvalidDecision.Error()})
		return
	}

	rep, err := h.store.GetReport(c.Request.Context(), reportID)
	if err != nil {
		if errors.Is(err, ErrReportNotFound) {
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "not_found", Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "internal_error", Message: err.Error()})
		return
	}
	if rep.Status != StatusPending {
		c.JSON(http.StatusConflict, auth.ErrorResponse{Error: "already_reviewed", Message: ErrReportResolved.Error()})
		return
	}

	// Determine target author before hiding so we can notify the content author without leaking body.
	contentAuthorID := h.lookupContentAuthor(c.Request.Context(), rep.TargetType, rep.TargetID)

	if decision == StatusApproved {
		if h.postStore == nil {
			c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "internal_error", Message: "post store unavailable"})
			return
		}
		var hideErr error
		switch rep.TargetType {
		case "post":
			hideErr = h.postStore.HidePost(c.Request.Context(), rep.TargetID)
		case "comment":
			hideErr = h.postStore.HideComment(c.Request.Context(), rep.TargetID)
		default:
			hideErr = ErrInvalidTargetType
		}
		if hideErr != nil {
			switch {
			case errors.Is(hideErr, posts.ErrPostNotFound), errors.Is(hideErr, posts.ErrCommentNotFound):
				c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "target_not_found", Message: hideErr.Error()})
			default:
				c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "hide_failed", Message: hideErr.Error()})
			}
			return
		}
	}

	resolved, err := h.store.ResolveReport(c.Request.Context(), admin.UserID, reportID, decision, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrReportNotFound):
			c.JSON(http.StatusNotFound, auth.ErrorResponse{Error: "not_found", Message: err.Error()})
		case errors.Is(err, ErrReportResolved):
			c.JSON(http.StatusConflict, auth.ErrorResponse{Error: "already_reviewed", Message: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "review_failed", Message: err.Error()})
		}
		return
	}

	h.notifyModerationOutcome(c.Request.Context(), resolved, contentAuthorID)
	c.JSON(http.StatusOK, resolved)
}

func (h *Handler) lookupContentAuthor(ctx context.Context, targetType, targetID string) string {
	if h.postStore == nil {
		return ""
	}
	switch targetType {
	case "post":
		if p, err := h.postStore.GetPost(ctx, targetID); err == nil {
			return p.AuthorID
		}
	case "comment":
		if cmt, err := h.postStore.GetComment(ctx, targetID); err == nil {
			return cmt.AuthorID
		}
	}
	return ""
}

func (h *Handler) notifyModerationOutcome(ctx context.Context, rep *Report, contentAuthorID string) {
	if h.notifStore == nil || rep == nil {
		return
	}

	// Never embed hidden content body or title in moderation notifications.
	reporterSummary := "您提交的举报已完成复核，暂未发现违规。"
	if rep.Status == StatusApproved {
		reporterSummary = "您提交的举报已核实并处理，相关内容已被管理员隐藏。"
	}
	if _, err := h.notifStore.AppendNotification(
		ctx,
		"moderation_reporter:"+rep.ID,
		rep.ReporterID,
		"moderation",
		"举报处理结果通知",
		reporterSummary,
		rep.TargetType,
		rep.TargetID,
	); err != nil {
		slog.Warn("failed to deliver moderation notification to reporter", "reportId", rep.ID, "error", err)
	}

	if rep.Status == StatusApproved && contentAuthorID != "" {
		authorSummary := "您发布的内容因违反社区规范已被管理员隐藏。"
		if _, err := h.notifStore.AppendNotification(
			ctx,
			"moderation_author:"+rep.ID,
			contentAuthorID,
			"moderation",
			"内容处置通知",
			authorSummary,
			rep.TargetType,
			rep.TargetID,
		); err != nil {
			slog.Warn("failed to deliver moderation notification to author", "reportId", rep.ID, "error", err)
		}
	}
}

// ListReports returns pending and reviewed reports for administrators in reverse chronological order.
// @ID communityListReports
// @Summary List content reports (Admin)
// @Tags Community
// @Produce json
// @Param status query string false "Optional status filter: pending, approved, rejected"
// @Param page query int false "Page number (default 1)"
// @Param pageSize query int false "Page size (default 20, max 100)"
// @Success 200 {object} ReportListResponse
// @Failure 400 {object} auth.ErrorResponse
// @Failure 401 {object} auth.ErrorResponse
// @Failure 403 {object} auth.ErrorResponse
// @Router /api/community/reports [get]
func (h *Handler) ListReports(c *gin.Context) {
	if _, ok := auth.RequireAdmin(c, h.resolver); !ok {
		return
	}

	statusFilter := strings.TrimSpace(c.Query("status"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	sizeParam := c.Query("pageSize")
	if sizeParam == "" {
		sizeParam = c.DefaultQuery("limit", "20")
	}
	pageSize, _ := strconv.Atoi(sizeParam)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	items, total, err := h.store.ListReports(c.Request.Context(), statusFilter, page, pageSize)
	if err != nil {
		if errors.Is(err, ErrInvalidStatusFilter) {
			c.JSON(http.StatusBadRequest, auth.ErrorResponse{Error: "invalid_status", Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, auth.ErrorResponse{Error: "list_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ReportListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
