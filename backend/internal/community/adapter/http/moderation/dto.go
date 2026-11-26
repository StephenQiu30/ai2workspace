package moderation

import (
	moderationdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/moderation"
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
	Items    []*moderationdomain.Report `json:"items"`
	Total    int                        `json:"total" example:"10"`
	Page     int                        `json:"page" example:"1"`
	PageSize int                        `json:"pageSize" example:"20"`
}
