package assistant

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes adds assistant endpoints to the application router.
func (m *Handler) RegisterRoutes(router gin.IRouter) {
	api := router.Group("/api/assistant")
	// Governance & Models
	api.GET("/governance/summary", m.handleGetGovernanceSummary)
	api.PATCH("/governance/budget", m.handleUpdateBudgetConfig)
	api.POST("/governance/pause", m.handlePauseAI)
	api.GET("/governance/reconciliations", m.handleListReconciliations)
	api.POST("/governance/reconciliations/:id/resolve", m.handleResolveReconciliation)

	api.GET("/models", m.handleListModels)
	api.PATCH("/models/:providerId/*modelId", m.handleUpdateModel)

	// Knowledge Base
	api.GET("/knowledge-bases", m.handleListKnowledgeBases)
	api.POST("/knowledge-bases", m.handleCreateKnowledgeBase)
	api.GET("/knowledge-bases/:id", m.handleGetKnowledgeBase)
	api.PATCH("/knowledge-bases/:id", m.handleUpdateKnowledgeBase)
	api.GET("/knowledge-bases/:id/sources", m.handleListKnowledgeSources)
	api.POST("/knowledge-bases/:id/sources", m.handleRegisterKnowledgeSource)
	api.PATCH("/knowledge-bases/:id/sources/:sourceId", m.handleUpdateKnowledgeSource)
	api.POST("/knowledge-sources/:id/index-jobs", m.handleStartIndexJob)
	api.GET("/index-jobs/:id", m.handleGetIndexJob)
	api.POST("/knowledge-sources/:id/publish", m.handlePublishKnowledgeSource)
	api.POST("/knowledge-sources/:id/revoke", m.handleRevokeKnowledgeSource)

	// Agents (Admin-only mutations per R-28)
	api.GET("/agents", m.handleListAgents)
	api.GET("/agents/:slug", m.handleGetAgentBySlug)
	api.POST("/agents", m.handleCreateAgent)
	api.PATCH("/agents/:id", m.handleUpdateAgent)
	api.POST("/agents/:id/publish", m.handlePublishAgent)
	api.POST("/agents/:id/disable", m.handleDisableAgent)

	// Sessions & Chat (Authenticated + emailVerified == true)
	api.GET("/sessions", m.handleListSessions)
	api.POST("/sessions", m.handleCreateSession)
	api.GET("/sessions/:id", m.handleGetSession)
	api.GET("/sessions/:id/messages", m.handleListMessages)
	api.POST("/sessions/:id/ask", m.handleAskQuestion)
	api.POST("/sessions/:id/runs/:runId/cancel", m.handleCancelRun)
}
