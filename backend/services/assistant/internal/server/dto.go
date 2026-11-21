package server

import (
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/knowledge"
)

// CreateKnowledgeBaseRequest defines input to create a knowledge base.
type CreateKnowledgeBaseRequest struct {
	Name        string               `json:"name" binding:"required" example:"Go 并发与微服务资料库"`
	Description string               `json:"description" example:"课程授权文档与常见问题"`
	Visibility  knowledge.Visibility `json:"visibility" binding:"required" example:"private"`
}

// UpdateKnowledgeBaseRequest defines input to update a knowledge base.
type UpdateKnowledgeBaseRequest struct {
	ExpectedVersion int                   `json:"expectedVersion" example:"1"`
	Name            *string               `json:"name,omitempty" example:"Go 并发资料库（更新）"`
	Description     *string               `json:"description,omitempty"`
	Visibility      *knowledge.Visibility `json:"visibility,omitempty" example:"public"`
}

// RegisterSourceRequest defines input to register a content source.
type RegisterSourceRequest struct {
	Kind      knowledge.SourceKind `json:"kind" binding:"required" example:"course_text"`
	SourceRef string               `json:"sourceRef" example:"crs_1001"`
	Title     string               `json:"title" binding:"required" example:"第 1 讲：并发模型"`
	Content   string               `json:"content" binding:"required"`
}

// UpdateSourceRequest defines input to update a source and bump its version.
type UpdateSourceRequest struct {
	Title   string `json:"title" example:"第 1 讲：并发模型（修订版）"`
	Content string `json:"content" binding:"required"`
}

// PublishSourceRequest defines input to publish an indexed source version.
type PublishSourceRequest struct {
	ExpectedVersion int `json:"expectedVersion" binding:"required" example:"1"`
}

// CreateAgentRequest defines input to create a draft agent configuration (admin only).
type CreateAgentRequest struct {
	Name             string   `json:"name" binding:"required" example:"平台课程助教"`
	Slug             string   `json:"slug" binding:"required" example:"go_course_tutor"`
	RoleInstruction  string   `json:"roleInstruction" binding:"required"`
	ProviderID       string   `json:"providerId" binding:"required" example:"deepseek"`
	ModelID          string   `json:"modelId" binding:"required" example:"deepseek-chat"`
	KnowledgeBaseIDs []string `json:"knowledgeBaseIds"`
}

// UpdateAgentRequest defines input to update an agent configuration and bump draftVersion (admin only).
type UpdateAgentRequest struct {
	ExpectedVersion  int       `json:"expectedVersion" example:"1"`
	Name             *string   `json:"name,omitempty"`
	Slug             *string   `json:"slug,omitempty"`
	RoleInstruction  *string   `json:"roleInstruction,omitempty"`
	ProviderID       *string   `json:"providerId,omitempty"`
	ModelID          *string   `json:"modelId,omitempty"`
	KnowledgeBaseIDs *[]string `json:"knowledgeBaseIds,omitempty"`
}

// DisableAgentRequest defines input to disable an agent (admin only).
type DisableAgentRequest struct {
	Reason string `json:"reason" example:"模型维护中，暂时停用"`
}

// CreateSessionRequest defines input to start a conversation session.
type CreateSessionRequest struct {
	AgentSlug string `json:"agentSlug" binding:"required" example:"customer_support"`
	CourseID  string `json:"courseId" example:"crs_1001"`
	Title     string `json:"title" example:"关于并发的疑问"`
}

// AskQuestionRequest defines input to ask a question with optional idempotency key.
type AskQuestionRequest struct {
	Question       string `json:"question" binding:"required" example:"请问 Go 的 Channel 如何避免死锁？"`
	IdempotencyKey string `json:"idempotencyKey,omitempty" example:"req_20261121_001"`
}

// SyncCourseKnowledgeRequest defines internal course knowledge sync payload.
type SyncCourseKnowledgeRequest struct {
	CourseID        string `json:"courseId" binding:"required" example:"crs_1001"`
	KnowledgeBaseID string `json:"knowledgeBaseId" binding:"required" example:"kb_123"`
	Title           string `json:"title" binding:"required" example:"课程图文同步"`
	Content         string `json:"content" binding:"required"`
}

// UpdateModelRequest defines admin updates to a model's enabled status or price version.
type UpdateModelRequest struct {
	DisplayName         *string    `json:"displayName,omitempty"`
	PriceVersion        *string    `json:"priceVersion,omitempty" example:"v2-2026-12"`
	PromptPricePerK     *int       `json:"promptPricePerK,omitempty" example:"1"`
	CompletionPricePerK *int       `json:"completionPricePerK,omitempty" example:"2"`
	EffectiveFrom       *time.Time `json:"effectiveFrom,omitempty"`
	ExpiresAt           *time.Time `json:"expiresAt,omitempty"`
	Enabled             *bool      `json:"enabled,omitempty"`
}

// UpdateBudgetConfigRequest defines admin budget configuration overrides.
type UpdateBudgetConfigRequest struct {
	UserDailyQuestions    *int `json:"userDailyQuestions,omitempty"`
	UserPerMinute         *int `json:"userPerMinute,omitempty"`
	UserConcurrency       *int `json:"userConcurrency,omitempty"`
	PlatformConcurrency   *int `json:"platformConcurrency,omitempty"`
	MaxSingleReserveCents *int `json:"maxSingleReserveCents,omitempty"`
	UserDailyCents        *int `json:"userDailyCents,omitempty"`
	PlatformDailyCents    *int `json:"platformDailyCents,omitempty"`
	PlatformMonthlyCents  *int `json:"platformMonthlyCents,omitempty"`
	BackgroundDailyCents  *int `json:"backgroundDailyCents,omitempty"`
	AlertThresholdPct     *int `json:"alertThresholdPct,omitempty"`
}

// PauseAIRequest defines admin toggle for pausing/resuming new AI requests.
type PauseAIRequest struct {
	Paused bool   `json:"paused"`
	Reason string `json:"reason" example:"上游供应商故障，临时暂停新 AI 请求"`
}

// ResolveReconciliationRequest defines admin resolution of an unknown-cost item.
type ResolveReconciliationRequest struct {
	ResolvedCents int    `json:"resolvedCents" example:"8"`
	Note          string `json:"note" example:"已核对供应商账单并确认实际费用"`
}

// StandardResponse wraps simple action confirmation responses.
type StandardResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ErrorResponse represents a structured API error response.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
