package assistant

import (
	"errors"
	"net/http"

	agentdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/agent"
	providerdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/provider"
	usagedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/usage"
	"github.com/gin-gonic/gin"
)

// @Summary 列出可见的 Agent 实例
// @Description 普通用户仅返回已发布实例且隐藏内部 systemPrompt；管理员返回全部（含草稿与停用）
// @ID assistantListAgents
// @Tags Agents
// @Produce json
// @Success 200 {array} agentdomain.Configuration
// @Failure 401 {object} ErrorResponse
// @Router /api/assistant/agents [get]
func (m *Handler) handleListAgents(c *gin.Context) {
	actor, ok := m.authenticate(c)
	if !ok {
		return
	}
	agents := m.agentService.ListAgents(c.Request.Context(), actor.IsAdmin())
	c.JSON(http.StatusOK, agents)
}

// @Summary 根据标识获取 Agent 实例详情
// @ID assistantGetAgentBySlug
// @Tags Agents
// @Produce json
// @Param slug path string true "Agent标识(如 customer_support 或 course_tutor)"
// @Success 200 {object} agentdomain.Configuration
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/agents/{slug} [get]
func (m *Handler) handleGetAgentBySlug(c *gin.Context) {
	actor, ok := m.authenticate(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	cfg, err := m.agentService.GetAgentBySlug(c.Request.Context(), slug, actor.IsAdmin())
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Code: http.StatusNotFound, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// @Summary 创建新 Agent 草稿配置（严格仅管理员，R-28）
// @ID assistantCreateAgent
// @Tags Agents
// @Accept json
// @Produce json
// @Param request body CreateAgentRequest true "创建请求"
// @Success 201 {object} agentdomain.Configuration
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/assistant/agents [post]
func (m *Handler) handleCreateAgent(c *gin.Context) {
	if _, ok := m.requireAdmin(c); !ok {
		return
	}
	var req CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	cfg, err := m.agentService.CreateAgent(
		c.Request.Context(), req.Name, req.Slug, req.RoleInstruction,
		req.ProviderID, req.ModelID, req.KnowledgeBaseIDs,
	)
	if err != nil {
		status := mapAgentErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cfg)
}

// @Summary 更新 Agent 配置并递增草稿版本（严格仅管理员，R-28）
// @ID assistantUpdateAgent
// @Tags Agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body UpdateAgentRequest true "更新请求"
// @Success 200 {object} agentdomain.Configuration
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/assistant/agents/{id} [patch]
func (m *Handler) handleUpdateAgent(c *gin.Context) {
	if _, ok := m.requireAdmin(c); !ok {
		return
	}
	id := c.Param("id")
	var req UpdateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	cfg, err := m.agentService.UpdateAgent(c.Request.Context(), id, agentdomain.UpdateAgentInput{
		ExpectedVersion:  req.ExpectedVersion,
		Name:             req.Name,
		Slug:             req.Slug,
		RoleInstruction:  req.RoleInstruction,
		ProviderID:       req.ProviderID,
		ModelID:          req.ModelID,
		KnowledgeBaseIDs: req.KnowledgeBaseIDs,
	})
	if err != nil {
		status := mapAgentErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// @Summary 发布 Agent 实例版本（严格仅管理员，R-28）
// @ID assistantPublishAgent
// @Tags Agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} agentdomain.Configuration
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/agents/{id}/publish [post]
func (m *Handler) handlePublishAgent(c *gin.Context) {
	if _, ok := m.requireAdmin(c); !ok {
		return
	}
	id := c.Param("id")
	cfg, err := m.agentService.PublishAgent(c.Request.Context(), id)
	if err != nil {
		status := mapAgentErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// @Summary 停用 Agent 实例并拒绝新问答（严格仅管理员，R-28）
// @ID assistantDisableAgent
// @Tags Agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body DisableAgentRequest false "停用原因"
// @Success 200 {object} agentdomain.Configuration
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/agents/{id}/disable [post]
func (m *Handler) handleDisableAgent(c *gin.Context) {
	if _, ok := m.requireAdmin(c); !ok {
		return
	}
	id := c.Param("id")
	var req DisableAgentRequest
	_ = c.ShouldBindJSON(&req)
	cfg, err := m.agentService.DisableAgent(c.Request.Context(), id, req.Reason)
	if err != nil {
		status := mapAgentErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// @Summary 获取当前用户的历史问答会话列表（需已验证邮箱）
// @ID assistantListSessions
// @Tags Chat
// @Produce json
// @Success 200 {array} agentdomain.Session
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/assistant/sessions [get]
func (m *Handler) handleListSessions(c *gin.Context) {
	actor, ok := m.requireVerifiedLearner(c)
	if !ok {
		return
	}
	sessions := m.agentService.ListSessions(c.Request.Context(), actor.UserID)
	c.JSON(http.StatusOK, sessions)
}

// @Summary 创建问答会话（需已验证邮箱）
// @ID assistantCreateSession
// @Tags Chat
// @Accept json
// @Produce json
// @Param request body CreateSessionRequest true "创建会话请求"
// @Success 201 {object} agentdomain.Session
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/sessions [post]
func (m *Handler) handleCreateSession(c *gin.Context) {
	actor, ok := m.requireVerifiedLearner(c)
	if !ok {
		return
	}
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	sess, err := m.agentService.CreateSession(c.Request.Context(), req.AgentSlug, actor.UserID, req.CourseID, req.Title)
	if err != nil {
		status := mapAgentErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sess)
}

// @Summary 获取本人会话详情（需已验证邮箱）
// @ID assistantGetSession
// @Tags Chat
// @Produce json
// @Param id path string true "会话ID"
// @Success 200 {object} agentdomain.Session
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/sessions/{id} [get]
func (m *Handler) handleGetSession(c *gin.Context) {
	actor, ok := m.requireVerifiedLearner(c)
	if !ok {
		return
	}
	id := c.Param("id")
	sess, err := m.agentService.GetSession(c.Request.Context(), id, actor.UserID)
	if err != nil {
		status := mapAgentErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, sess)
}

// @Summary 列出指定会话的消息历史并实时复核知识引用权限（需已验证邮箱）
// @ID assistantListSessionMessages
// @Tags Chat
// @Produce json
// @Param id path string true "会话ID"
// @Success 200 {array} agentdomain.Message
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/sessions/{id}/messages [get]
func (m *Handler) handleListMessages(c *gin.Context) {
	actor, ok := m.requireVerifiedLearner(c)
	if !ok {
		return
	}
	id := c.Param("id")
	msgs, err := m.agentService.ListMessages(c.Request.Context(), id, actor.UserID)
	if err != nil {
		status := mapAgentErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, msgs)
}

// @Summary 向 Agent 提问（强制联网搜索、授权 RAG 召回、多轮上下文、幂等与动态预算治理）
// @ID assistantAskQuestion
// @Tags Chat
// @Accept json
// @Produce json
// @Param id path string true "会话ID"
// @Param request body AskQuestionRequest true "提问内容"
// @Success 200 {object} agentdomain.Message
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 402 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/assistant/sessions/{id}/ask [post]
func (m *Handler) handleAskQuestion(c *gin.Context) {
	actor, ok := m.requireVerifiedLearner(c)
	if !ok {
		return
	}
	id := c.Param("id")
	var req AskQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	msg, err := m.agentService.AskWithOptions(c.Request.Context(), id, actor.UserID, req.Question, req.IdempotencyKey)
	if err != nil {
		status := mapAgentErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, msg)
}

// @Summary 取消正在执行的问答运行（需已验证邮箱且为本人会话）
// @ID assistantCancelRun
// @Tags Chat
// @Produce json
// @Param id path string true "会话ID"
// @Param runId path string true "运行ID"
// @Success 200 {object} agentdomain.Run
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/sessions/{id}/runs/{runId}/cancel [post]
func (m *Handler) handleCancelRun(c *gin.Context) {
	actor, ok := m.requireVerifiedLearner(c)
	if !ok {
		return
	}
	sessionID := c.Param("id")
	runID := c.Param("runId")
	run, err := m.agentService.CancelRun(c.Request.Context(), sessionID, runID, actor.UserID)
	if err != nil {
		status := mapAgentErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, run)
}

func mapAgentErrorStatus(err error) int {
	switch {
	case errors.Is(err, agentdomain.ErrAgentNotFound),
		errors.Is(err, agentdomain.ErrSessionNotFound),
		errors.Is(err, agentdomain.ErrRunNotFound):
		return http.StatusNotFound
	case errors.Is(err, agentdomain.ErrUnauthorizedAccess),
		errors.Is(err, agentdomain.ErrAgentDisabled),
		errors.Is(err, agentdomain.ErrCitationAccessRevoked):
		return http.StatusForbidden
	case errors.Is(err, agentdomain.ErrSlugAlreadyExists),
		errors.Is(err, agentdomain.ErrVersionMismatch),
		errors.Is(err, agentdomain.ErrIdempotencyConflict),
		errors.Is(err, agentdomain.ErrRunInProgress):
		return http.StatusConflict
	case errors.Is(err, usagedomain.ErrRateLimitExceeded),
		errors.Is(err, usagedomain.ErrUserConcurrencyLimit),
		errors.Is(err, usagedomain.ErrPlatformConcurrency),
		errors.Is(err, usagedomain.ErrDailyLimitExceeded):
		return http.StatusTooManyRequests
	case errors.Is(err, usagedomain.ErrUserBudgetExceeded),
		errors.Is(err, usagedomain.ErrPlatformBudget),
		errors.Is(err, usagedomain.ErrPlatformMonthlyBudget),
		errors.Is(err, usagedomain.ErrSingleRequestBudgetExceeded):
		return http.StatusPaymentRequired
	case errors.Is(err, usagedomain.ErrAIPaused),
		errors.Is(err, providerdomain.ErrProviderNotConfigured),
		errors.Is(err, providerdomain.ErrSearchNotConfigured),
		errors.Is(err, providerdomain.ErrModelPriceExpired):
		return http.StatusServiceUnavailable
	case errors.Is(err, providerdomain.ErrSearchFailed),
		errors.Is(err, providerdomain.ErrSearchAuthFailed),
		errors.Is(err, providerdomain.ErrSearchTimeout),
		errors.Is(err, providerdomain.ErrModelGenerationFailed):
		return http.StatusBadGateway
	default:
		return http.StatusBadRequest
	}
}
