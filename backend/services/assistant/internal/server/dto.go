package server

import (
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/knowledge"
)

// CreateKnowledgeBaseRequest defines input to create base.
type CreateKnowledgeBaseRequest struct {
	Name        string               `json:"name" binding:"required"`
	Description string               `json:"description"`
	Visibility  knowledge.Visibility `json:"visibility" binding:"required"`
}

// RegisterSourceRequest defines input to register content source.
type RegisterSourceRequest struct {
	Kind      knowledge.SourceKind `json:"kind" binding:"required"`
	SourceRef string               `json:"sourceRef"`
	Title     string               `json:"title" binding:"required"`
	Content   string               `json:"content" binding:"required"`
}

// PublishSourceRequest defines input to publish source.
type PublishSourceRequest struct {
	ExpectedVersion int `json:"expectedVersion" binding:"required"`
}

// CreateAgentRequest defines input to create an agent.
type CreateAgentRequest struct {
	Name             string   `json:"name" binding:"required"`
	Slug             string   `json:"slug" binding:"required"`
	RoleInstruction  string   `json:"roleInstruction" binding:"required"`
	ProviderID       string   `json:"providerId" binding:"required"`
	ModelID          string   `json:"modelId" binding:"required"`
	KnowledgeBaseIDs []string `json:"knowledgeBaseIds"`
}

// CreateSessionRequest defines input to start conversation.
type CreateSessionRequest struct {
	AgentSlug string `json:"agentSlug" binding:"required"`
	CourseID  string `json:"courseId"`
	Title     string `json:"title"`
}

// AskQuestionRequest defines input to ask a question.
type AskQuestionRequest struct {
	Question string `json:"question" binding:"required"`
}

// SyncCourseKnowledgeRequest defines internal sync input.
type SyncCourseKnowledgeRequest struct {
	CourseID        string `json:"courseId" binding:"required"`
	KnowledgeBaseID string `json:"knowledgeBaseId" binding:"required"`
	Title           string `json:"title" binding:"required"`
	Content         string `json:"content" binding:"required"`
}

// StandardResponse wraps API responses.
type StandardResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
