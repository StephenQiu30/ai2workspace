// Package server provides HTTP routing and handlers for the assistant service.
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/assistant/api"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/agent"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/health"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/knowledge"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/provider"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/usage"
)

// Server coordinates HTTP dependencies.
type Server struct {
	cfg              *config.Config
	router           *gin.Engine
	usageService     *usage.Service
	knowledgeService *knowledge.Service
	modelClient      *provider.Client
	searchEngine     provider.SearchEngine
	agentService     *agent.Service
}

// New creates and configures the HTTP server.
func New(cfg *config.Config) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	usageSvc := usage.NewService()
	knowledgeSvc := knowledge.NewService(nil)
	modelCli := provider.NewClient()
	searchEng := provider.NewSearchEngine()
	agentSvc := agent.NewService(usageSvc, knowledgeSvc, modelCli, searchEng)

	s := &Server{
		cfg:              cfg,
		router:           router,
		usageService:     usageSvc,
		knowledgeService: knowledgeSvc,
		modelClient:      modelCli,
		searchEngine:     searchEng,
		agentService:     agentSvc,
	}

	s.setupRoutes()
	return s
}

// Router returns the underlying gin Engine.
func (s *Server) Router() *gin.Engine {
	return s.router
}

func (s *Server) getUserID(c *gin.Context) string {
	if uid := c.GetHeader("X-User-ID"); uid != "" {
		return uid
	}
	if cookie, err := c.Cookie("ai2workspace_session"); err == nil && cookie != "" {
		return "usr_" + cookie[:8]
	}
	return "usr_guest"
}

func (s *Server) setupRoutes() {
	if s.cfg.SwaggerEnabled {
		s.router.GET("/swagger/assistant/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	s.router.GET("/healthz", health.Get)
	s.router.GET("/api/assistant/health", health.Get)

	api := s.router.Group("/api/assistant")
	{
		// Governance & Models
		api.GET("/governance/summary", s.handleGetGovernanceSummary)
		api.GET("/models", s.handleListModels)

		// Knowledge Base
		api.GET("/knowledge-bases", s.handleListKnowledgeBases)
		api.POST("/knowledge-bases", s.handleCreateKnowledgeBase)
		api.GET("/knowledge-bases/:id", s.handleGetKnowledgeBase)
		api.GET("/knowledge-bases/:id/sources", s.handleListKnowledgeSources)
		api.POST("/knowledge-bases/:id/sources", s.handleRegisterKnowledgeSource)
		api.POST("/knowledge-sources/:id/index-jobs", s.handleStartIndexJob)
		api.POST("/knowledge-sources/:id/publish", s.handlePublishKnowledgeSource)
		api.POST("/knowledge-sources/:id/revoke", s.handleRevokeKnowledgeSource)

		// Agents & Sessions
		api.GET("/agents", s.handleListAgents)
		api.GET("/agents/:slug", s.handleGetAgentBySlug)
		api.POST("/agents", s.handleCreateAgent)
		api.POST("/agents/:id/publish", s.handlePublishAgent)

		api.POST("/sessions", s.handleCreateSession)
		api.GET("/sessions/:id", s.handleGetSession)
		api.GET("/sessions/:id/messages", s.handleListMessages)
		api.POST("/sessions/:id/ask", s.handleAskQuestion)
	}

	// Internal APIs
	internal := s.router.Group("/internal/assistant")
	{
		internal.POST("/knowledge/sync-course", s.handleSyncCourseKnowledge)
	}
}

// @Summary 获取当前用户的 AI 问答额度与用量治理概况
// @Description 返回今日成功问答次数、单账号今日预算、平台总体预算与活跃并发
// @Tags Governance
// @Produce json
// @Success 200 {object} usage.UsageSummary
// @Router /api/assistant/governance/summary [get]
func (s *Server) handleGetGovernanceSummary(c *gin.Context) {
	userID := s.getUserID(c)
	summary := s.usageService.GetSummary(c.Request.Context(), userID)
	c.JSON(http.StatusOK, summary)
}

// @Summary 获取允许的模型目录
// @Description 返回四家供应商（DeepSeek、OpenAI、OpenRouter、火山引擎）的允许模型及定价
// @Tags Governance
// @Produce json
// @Success 200 {array} provider.ModelInfo
// @Router /api/assistant/models [get]
func (s *Server) handleListModels(c *gin.Context) {
	models := s.modelClient.ListCatalog()
	c.JSON(http.StatusOK, models)
}

// @Summary 列出可见的知识库列表
// @Tags Knowledge
// @Produce json
// @Success 200 {array} knowledge.KnowledgeBase
// @Router /api/assistant/knowledge-bases [get]
func (s *Server) handleListKnowledgeBases(c *gin.Context) {
	userID := s.getUserID(c)
	bases := s.knowledgeService.ListKnowledgeBases(c.Request.Context(), userID, true)
	c.JSON(http.StatusOK, bases)
}

// @Summary 创建新知识库
// @Tags Knowledge
// @Accept json
// @Produce json
// @Param request body CreateKnowledgeBaseRequest true "创建知识库请求"
// @Success 201 {object} knowledge.KnowledgeBase
// @Router /api/assistant/knowledge-bases [post]
func (s *Server) handleCreateKnowledgeBase(c *gin.Context) {
	var req CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	userID := s.getUserID(c)
	kb, err := s.knowledgeService.CreateKnowledgeBase(c.Request.Context(), userID, req.Name, req.Description, req.Visibility)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Code: 500, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, kb)
}

// @Summary 获取知识库详情
// @Tags Knowledge
// @Produce json
// @Param id path string true "知识库ID"
// @Success 200 {object} knowledge.KnowledgeBase
// @Router /api/assistant/knowledge-bases/{id} [get]
func (s *Server) handleGetKnowledgeBase(c *gin.Context) {
	id := c.Param("id")
	kb, err := s.knowledgeService.GetKnowledgeBase(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Code: 404, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, kb)
}

// @Summary 列出知识库内的来源列表
// @Tags Knowledge
// @Produce json
// @Param id path string true "知识库ID"
// @Success 200 {array} knowledge.KnowledgeSource
// @Router /api/assistant/knowledge-bases/{id}/sources [get]
func (s *Server) handleListKnowledgeSources(c *gin.Context) {
	id := c.Param("id")
	sources := s.knowledgeService.ListSources(c.Request.Context(), id)
	c.JSON(http.StatusOK, sources)
}

// @Summary 在知识库内登记新来源（课程图文或文档）
// @Tags Knowledge
// @Accept json
// @Produce json
// @Param id path string true "知识库ID"
// @Param request body RegisterSourceRequest true "登记来源请求"
// @Success 201 {object} knowledge.KnowledgeSource
// @Router /api/assistant/knowledge-bases/{id}/sources [post]
func (s *Server) handleRegisterKnowledgeSource(c *gin.Context) {
	id := c.Param("id")
	var req RegisterSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	userID := s.getUserID(c)
	src, err := s.knowledgeService.RegisterKnowledgeSource(c.Request.Context(), id, userID, req.Kind, req.SourceRef, req.Title, req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, src)
}

// @Summary 发起知识来源切片与入库索引作业
// @Tags Knowledge
// @Produce json
// @Param id path string true "来源ID"
// @Success 200 {object} knowledge.IndexJob
// @Router /api/assistant/knowledge-sources/{id}/index-jobs [post]
func (s *Server) handleStartIndexJob(c *gin.Context) {
	id := c.Param("id")
	job, err := s.knowledgeService.StartIndexJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

// @Summary 发布知识来源使其可被检索
// @Tags Knowledge
// @Accept json
// @Produce json
// @Param id path string true "来源ID"
// @Param request body PublishSourceRequest true "发布版本请求"
// @Success 200 {object} StandardResponse
// @Router /api/assistant/knowledge-sources/{id}/publish [post]
func (s *Server) handlePublishKnowledgeSource(c *gin.Context) {
	id := c.Param("id")
	var req PublishSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	if err := s.knowledgeService.PublishKnowledgeSource(c.Request.Context(), id, req.ExpectedVersion); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StandardResponse{Code: 200, Message: "source published successfully"})
}

// @Summary 立即撤回知识来源（检索将阻断召回，绝不泄露）
// @Tags Knowledge
// @Produce json
// @Param id path string true "来源ID"
// @Success 200 {object} StandardResponse
// @Router /api/assistant/knowledge-sources/{id}/revoke [post]
func (s *Server) handleRevokeKnowledgeSource(c *gin.Context) {
	id := c.Param("id")
	if err := s.knowledgeService.RevokeKnowledgeSource(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StandardResponse{Code: 200, Message: "source revoked successfully"})
}

// @Summary 列出所有 Agent 实例
// @Tags Agents
// @Produce json
// @Success 200 {array} agent.Configuration
// @Router /api/assistant/agents [get]
func (s *Server) handleListAgents(c *gin.Context) {
	agents := s.agentService.ListAgents(c.Request.Context())
	c.JSON(http.StatusOK, agents)
}

// @Summary 根据标识获取 Agent 实例详情
// @Tags Agents
// @Produce json
// @Param slug path string true "Agent标识(如 customer_support 或 course_tutor)"
// @Success 200 {object} agent.Configuration
// @Router /api/assistant/agents/{slug} [get]
func (s *Server) handleGetAgentBySlug(c *gin.Context) {
	slug := c.Param("slug")
	cfg, err := s.agentService.GetAgentBySlug(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Code: 404, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// @Summary 创建新 Agent 配置
// @Tags Agents
// @Accept json
// @Produce json
// @Param request body CreateAgentRequest true "创建请求"
// @Success 201 {object} agent.Configuration
// @Router /api/assistant/agents [post]
func (s *Server) handleCreateAgent(c *gin.Context) {
	var req CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	cfg, err := s.agentService.CreateAgent(c.Request.Context(), req.Name, req.Slug, req.RoleInstruction, req.ProviderID, req.ModelID, req.KnowledgeBaseIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cfg)
}

// @Summary 发布 Agent 实例版本
// @Tags Agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} agent.Configuration
// @Router /api/assistant/agents/{id}/publish [post]
func (s *Server) handlePublishAgent(c *gin.Context) {
	id := c.Param("id")
	cfg, err := s.agentService.PublishAgent(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// @Summary 创建问答会话
// @Tags Chat
// @Accept json
// @Produce json
// @Param request body CreateSessionRequest true "创建会话请求"
// @Success 201 {object} agent.Session
// @Router /api/assistant/sessions [post]
func (s *Server) handleCreateSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	userID := s.getUserID(c)
	sess, err := s.agentService.CreateSession(c.Request.Context(), req.AgentSlug, userID, req.CourseID, req.Title)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sess)
}

// @Summary 获取会话详情
// @Tags Chat
// @Produce json
// @Param id path string true "会话ID"
// @Success 200 {object} agent.Session
// @Router /api/assistant/sessions/{id} [get]
func (s *Server) handleGetSession(c *gin.Context) {
	id := c.Param("id")
	userID := s.getUserID(c)
	sess, err := s.agentService.GetSession(c.Request.Context(), id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Code: 404, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, sess)
}

// @Summary 列出指定会话的消息历史
// @Tags Chat
// @Produce json
// @Param id path string true "会话ID"
// @Success 200 {array} agent.Message
// @Router /api/assistant/sessions/{id}/messages [get]
func (s *Server) handleListMessages(c *gin.Context) {
	id := c.Param("id")
	userID := s.getUserID(c)
	msgs, err := s.agentService.ListMessages(c.Request.Context(), id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Code: 404, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, msgs)
}

// @Summary 向 Agent 提问（执行强制真实联网搜索、授权 RAG 召回与预算结算）
// @Tags Chat
// @Accept json
// @Produce json
// @Param id path string true "会话ID"
// @Param request body AskQuestionRequest true "提问内容"
// @Success 200 {object} agent.Message
// @Router /api/assistant/sessions/{id}/ask [post]
func (s *Server) handleAskQuestion(c *gin.Context) {
	id := c.Param("id")
	var req AskQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	userID := s.getUserID(c)
	msg, err := s.agentService.Ask(c.Request.Context(), id, userID, req.Question)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, msg)
}

// @Summary 内部接口：课程图文知识同步
// @Tags Internal
// @Accept json
// @Produce json
// @Param request body SyncCourseKnowledgeRequest true "同步课程内容"
// @Success 200 {object} StandardResponse
// @Router /internal/assistant/knowledge/sync-course [post]
func (s *Server) handleSyncCourseKnowledge(c *gin.Context) {
	var req SyncCourseKnowledgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	src, err := s.knowledgeService.RegisterKnowledgeSource(c.Request.Context(), req.KnowledgeBaseID, "system", knowledge.KindCourseText, req.CourseID, req.Title, req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: 400, Message: err.Error()})
		return
	}
	_, _ = s.knowledgeService.StartIndexJob(c.Request.Context(), src.ID)
	_ = s.knowledgeService.PublishKnowledgeSource(c.Request.Context(), src.ID, src.SourceVersion)

	c.JSON(http.StatusOK, StandardResponse{Code: 200, Message: "course knowledge synced"})
}
