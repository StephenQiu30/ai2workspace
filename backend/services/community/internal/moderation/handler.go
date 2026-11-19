package moderation

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// CreateReportRequest defines payload to flag violation content.
type CreateReportRequest struct {
	TargetType string `json:"targetType" binding:"required"` // "post", "comment"
	TargetID   string `json:"targetId" binding:"required"`
	Reason     string `json:"reason" binding:"required"`
}

// ModerateReportRequest defines admin decision payload.
type ModerateReportRequest struct {
	Decision string `json:"decision" binding:"required"` // "approved", "rejected"
	Reason   string `json:"reason"`
}

// ReportListResponse represents paginated reports list for admin console.
type ReportListResponse struct {
	Items []*Report `json:"items"`
	Total int       `json:"total"`
	Page  int       `json:"page"`
	Limit int       `json:"limit"`
}

// Handler manages moderation HTTP endpoints.
type Handler struct {
	store Store
}

// NewHandler creates a moderation Handler.
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// CreateReport flags a post or comment for violation review.
// @ID communityCreateReport
// @Summary Report post or comment
// @Tags Community
// @Accept json
// @Produce json
// @Param request body CreateReportRequest true "Report payload"
// @Success 201 {object} Report
// @Failure 400 {object} map[string]string
// @Router /api/community/reports [post]
func (h *Handler) CreateReport(c *gin.Context) {
	var req CreateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reporterID := c.GetHeader("X-User-ID")
	if reporterID == "" {
		reporterID = "mock_reporter"
	}

	report, err := h.store.CreateReport(c.Request.Context(), reporterID, req.TargetType, req.TargetID, req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, report)
}

// ModerateReport executes admin approval or rejection of a report.
// @ID communityModerateReport
// @Summary Review report and moderate content (Admin)
// @Description Approving a report automatically hides the target content to prevent further leaks.
// @Tags Community
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body ModerateReportRequest true "Moderation decision"
// @Success 200 {object} Report
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/community/reports/{id}/review [post]
func (h *Handler) ModerateReport(c *gin.Context) {
	reportID := c.Param("id")
	var req ModerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reviewerID := c.GetHeader("X-User-ID")
	if reviewerID == "" {
		reviewerID = "admin_moderator"
	}

	approved := req.Decision == "approved"
	report, err := h.store.ModerateReport(c.Request.Context(), reviewerID, reportID, approved, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrReportNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, report)
}

// ListReports returns pending and reviewed reports for admin console.
// @ID communityListReports
// @Summary List content reports (Admin)
// @Tags Community
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Success 200 {object} ReportListResponse
// @Router /api/community/reports [get]
func (h *Handler) ListReports(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	items, total, err := h.store.ListReports(c.Request.Context(), page, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ReportListResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}
