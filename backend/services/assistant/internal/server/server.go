// Package server provides HTTP routing, trusted session authentication, strict RBAC, and handlers for the assistant service.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/StephenQiu30/ai2workspace/backend/services/assistant/api"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/agent"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/health"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/knowledge"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/provider"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/storage"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/usage"
)

// Dependencies allows tests and callers to inject explicit collaborators without trusting unverified headers.
type Dependencies struct {
	DBPool              *pgxpool.Pool
	SessionResolver     SessionResolver
	CourseAccessChecker knowledge.UserAccessChecker
	UsageService        *usage.Service
	KnowledgeService    *knowledge.Service
	ModelClient         *provider.Client
	SearchEngine        provider.SearchEngine
	AgentService        *agent.Service
}

// Server coordinates HTTP routing and domain services.
type Server struct {
	cfg              *config.Config
	router           *gin.Engine
	dbPool           *pgxpool.Pool
	ownsDBPool       bool
	sessionResolver  SessionResolver
	usageService     *usage.Service
	knowledgeService *knowledge.Service
	modelClient      *provider.Client
	searchEngine     provider.SearchEngine
	agentService     *agent.Service
}

// New creates and configures the HTTP server from runtime configuration.
func New(cfg *config.Config) *Server {
	return NewWithDeps(cfg, Dependencies{})
}

// NewWithDeps creates and configures the HTTP server with optional injected dependencies.
func NewWithDeps(cfg *config.Config, deps Dependencies) *Server {
	if cfg == nil {
		cfg, _ = config.Load()
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	pool := deps.DBPool
	ownsPool := false
	if pool == nil && cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		opened, err := storage.OpenPool(ctx, cfg.DatabaseURL)
		cancel()
		if err != nil {
			slog.Error("failed to connect postgres; falling back to in-memory store", "service", "assistant", "error", err)
		} else {
			pool = opened
			ownsPool = true
		}
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resolver := deps.SessionResolver
	if resolver == nil {
		resolver = newHTTPSessionResolver(cfg.IdentityServiceURL, cfg.ServiceToken, httpClient)
	}

	accessChecker := deps.CourseAccessChecker
	if accessChecker == nil {
		accessChecker = newHTTPCourseAccessChecker(cfg.LearningServiceURL, cfg.ServiceToken, httpClient)
	}

	usageSvc := deps.UsageService
	if usageSvc == nil {
		usageSvc = usage.NewServiceWithPool(pool)
	}

	knowledgeSvc := deps.KnowledgeService
	if knowledgeSvc == nil {
		knowledgeSvc = knowledge.NewServiceWithPool(accessChecker, usageSvc, pool)
	}

	modelCli := deps.ModelClient
	if modelCli == nil {
		modelCli = provider.NewClientWithOptions(provider.ClientOptions{
			Endpoints: map[string]config.ProviderEndpoint{
				"deepseek":   cfg.DeepSeek,
				"openrouter": cfg.OpenRouter,
				"volcengine": cfg.Volcengine,
				"openai":     cfg.OpenAI,
			},
			AllowLocalSimulation: cfg.AllowLocalSimulation,
			DBPool:               pool,
		})
	}

	searchEng := deps.SearchEngine
	if searchEng == nil {
		searchEng = provider.NewSearchEngine(cfg)
	}

	agentSvc := deps.AgentService
	if agentSvc == nil {
		agentSvc = agent.NewServiceWithPool(usageSvc, knowledgeSvc, modelCli, searchEng, pool)
	}

	s := &Server{
		cfg:              cfg,
		router:           router,
		dbPool:           pool,
		ownsDBPool:       ownsPool,
		sessionResolver:  resolver,
		usageService:     usageSvc,
		knowledgeService: knowledgeSvc,
		modelClient:      modelCli,
		searchEngine:     searchEng,
		agentService:     agentSvc,
	}

	s.setupRoutes()
	return s
}

// Close releases resources owned by the server (such as PostgreSQL connection pool).
func (s *Server) Close() {
	if s.ownsDBPool && s.dbPool != nil {
		s.dbPool.Close()
	}
}

// Router returns the underlying gin Engine.
func (s *Server) Router() *gin.Engine {
	return s.router
}

// Services accessors for tests and diagnostics.
func (s *Server) UsageService() *usage.Service         { return s.usageService }
func (s *Server) KnowledgeService() *knowledge.Service { return s.knowledgeService }
func (s *Server) ModelClient() *provider.Client        { return s.modelClient }
func (s *Server) AgentService() *agent.Service         { return s.agentService }

func (s *Server) authenticate(c *gin.Context) (*Actor, bool) {
	cookie, err := c.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie) == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Code: http.StatusUnauthorized, Message: ErrUnauthorized.Error()})
		return nil, false
	}
	actor, err := s.sessionResolver(c.Request.Context(), cookie)
	if err != nil {
		if errors.Is(err, ErrAccountSuspended) {
			c.JSON(http.StatusForbidden, ErrorResponse{Code: http.StatusForbidden, Message: ErrAccountSuspended.Error()})
			return nil, false
		}
		c.JSON(http.StatusUnauthorized, ErrorResponse{Code: http.StatusUnauthorized, Message: ErrUnauthorized.Error()})
		return nil, false
	}
	if actor == nil || strings.TrimSpace(actor.UserID) == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Code: http.StatusUnauthorized, Message: ErrUnauthorized.Error()})
		return nil, false
	}
	if strings.EqualFold(actor.AccountStatus, "suspended") {
		c.JSON(http.StatusForbidden, ErrorResponse{Code: http.StatusForbidden, Message: ErrAccountSuspended.Error()})
		return nil, false
	}
	return actor, true
}

func (s *Server) requireAdmin(c *gin.Context) (*Actor, bool) {
	actor, ok := s.authenticate(c)
	if !ok {
		return nil, false
	}
	if !actor.IsAdmin() {
		c.JSON(http.StatusForbidden, ErrorResponse{Code: http.StatusForbidden, Message: "admin role required"})
		return nil, false
	}
	return actor, true
}

func (s *Server) requireKnowledgeMaintainer(c *gin.Context) (*Actor, bool) {
	actor, ok := s.authenticate(c)
	if !ok {
		return nil, false
	}
	if !actor.CanManageKnowledge() {
		c.JSON(http.StatusForbidden, ErrorResponse{Code: http.StatusForbidden, Message: "admin or instructor role required"})
		return nil, false
	}
	return actor, true
}

func (s *Server) requireVerifiedLearner(c *gin.Context) (*Actor, bool) {
	actor, ok := s.authenticate(c)
	if !ok {
		return nil, false
	}
	if !actor.EmailVerified {
		c.JSON(http.StatusForbidden, ErrorResponse{Code: http.StatusForbidden, Message: ErrEmailNotVerified.Error()})
		return nil, false
	}
	return actor, true
}

func (s *Server) verifyServiceToken(c *gin.Context) bool {
	token := strings.TrimSpace(c.GetHeader("X-Service-Token"))
	if token == "" || s.cfg.ServiceToken == "" || token != s.cfg.ServiceToken {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Code: http.StatusUnauthorized, Message: "valid X-Service-Token required"})
		return false
	}
	return true
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
		api.PATCH("/governance/budget", s.handleUpdateBudgetConfig)
		api.POST("/governance/pause", s.handlePauseAI)
		api.GET("/governance/reconciliations", s.handleListReconciliations)
		api.POST("/governance/reconciliations/:id/resolve", s.handleResolveReconciliation)

		api.GET("/models", s.handleListModels)
		api.PATCH("/models/:providerId/*modelId", s.handleUpdateModel)

		// Knowledge Base
		api.GET("/knowledge-bases", s.handleListKnowledgeBases)
		api.POST("/knowledge-bases", s.handleCreateKnowledgeBase)
		api.GET("/knowledge-bases/:id", s.handleGetKnowledgeBase)
		api.PATCH("/knowledge-bases/:id", s.handleUpdateKnowledgeBase)
		api.GET("/knowledge-bases/:id/sources", s.handleListKnowledgeSources)
		api.POST("/knowledge-bases/:id/sources", s.handleRegisterKnowledgeSource)
		api.PATCH("/knowledge-bases/:id/sources/:sourceId", s.handleUpdateKnowledgeSource)
		api.POST("/knowledge-sources/:id/index-jobs", s.handleStartIndexJob)
		api.GET("/index-jobs/:id", s.handleGetIndexJob)
		api.POST("/knowledge-sources/:id/publish", s.handlePublishKnowledgeSource)
		api.POST("/knowledge-sources/:id/revoke", s.handleRevokeKnowledgeSource)

		// Agents (Admin-only mutations per R-28)
		api.GET("/agents", s.handleListAgents)
		api.GET("/agents/:slug", s.handleGetAgentBySlug)
		api.POST("/agents", s.handleCreateAgent)
		api.PATCH("/agents/:id", s.handleUpdateAgent)
		api.POST("/agents/:id/publish", s.handlePublishAgent)
		api.POST("/agents/:id/disable", s.handleDisableAgent)

		// Sessions & Chat (Authenticated + emailVerified == true)
		api.GET("/sessions", s.handleListSessions)
		api.POST("/sessions", s.handleCreateSession)
		api.GET("/sessions/:id", s.handleGetSession)
		api.GET("/sessions/:id/messages", s.handleListMessages)
		api.POST("/sessions/:id/ask", s.handleAskQuestion)
		api.POST("/sessions/:id/runs/:runId/cancel", s.handleCancelRun)
	}

	// Internal APIs (Strictly X-Service-Token authenticated)
	internal := s.router.Group("/internal/assistant")
	{
		internal.POST("/knowledge/sync-course", s.handleSyncCourseKnowledge)
	}
}

// @Summary 获取当前用户的 AI 问答额度与用量治理概况
// @Description 返回本人今日成功次数与预算占用；管理员额外返回平台日月预算、后台子额度、未知费用与 80% 告警标志
// @ID assistantGetGovernanceSummary
// @Tags Governance
// @Produce json
// @Success 200 {object} usage.UsageSummary
// @Failure 401 {object} ErrorResponse
// @Router /api/assistant/governance/summary [get]
func (s *Server) handleGetGovernanceSummary(c *gin.Context) {
	actor, ok := s.authenticate(c)
	if !ok {
		return
	}
	summary := s.usageService.GetSummary(c.Request.Context(), actor.UserID, actor.IsAdmin())
	c.JSON(http.StatusOK, summary)
}

// @Summary 更新平台与账号 AI 预算限额配置（仅管理员）
// @ID assistantUpdateBudgetConfig
// @Tags Governance
// @Accept json
// @Produce json
// @Param request body UpdateBudgetConfigRequest true "预算配置更新请求"
// @Success 200 {object} usage.BudgetConfig
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/assistant/governance/budget [patch]
func (s *Server) handleUpdateBudgetConfig(c *gin.Context) {
	if _, ok := s.requireAdmin(c); !ok {
		return
	}
	var req UpdateBudgetConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	cfg, err := s.usageService.UpdateConfig(c.Request.Context(), usage.BudgetConfigUpdate{
		UserDailyQuestions:    req.UserDailyQuestions,
		UserPerMinute:         req.UserPerMinute,
		UserConcurrency:       req.UserConcurrency,
		PlatformConcurrency:   req.PlatformConcurrency,
		MaxSingleReserveCents: req.MaxSingleReserveCents,
		UserDailyCents:        req.UserDailyCents,
		PlatformDailyCents:    req.PlatformDailyCents,
		PlatformMonthlyCents:  req.PlatformMonthlyCents,
		BackgroundDailyCents:  req.BackgroundDailyCents,
		AlertThresholdPct:     req.AlertThresholdPct,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// @Summary 暂停或恢复平台新 AI 请求准入（仅管理员）
// @ID assistantSetAIPause
// @Tags Governance
// @Accept json
// @Produce json
// @Param request body PauseAIRequest true "暂停开关请求"
// @Success 200 {object} usage.BudgetConfig
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/assistant/governance/pause [post]
func (s *Server) handlePauseAI(c *gin.Context) {
	if _, ok := s.requireAdmin(c); !ok {
		return
	}
	var req PauseAIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	cfg, err := s.usageService.SetPaused(c.Request.Context(), req.Paused, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Code: http.StatusInternalServerError, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// @Summary 列出待核对及历史未知费用记录（仅管理员）
// @ID assistantListReconciliations
// @Tags Governance
// @Produce json
// @Success 200 {array} usage.ReconciliationItem
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/assistant/governance/reconciliations [get]
func (s *Server) handleListReconciliations(c *gin.Context) {
	if _, ok := s.requireAdmin(c); !ok {
		return
	}
	items := s.usageService.ListReconciliations(c.Request.Context())
	c.JSON(http.StatusOK, items)
}

// @Summary 人工核对并结清未知费用条目（仅管理员）
// @ID assistantResolveReconciliation
// @Tags Governance
// @Accept json
// @Produce json
// @Param id path string true "未知费用核对ID"
// @Param request body ResolveReconciliationRequest true "核对结果"
// @Success 200 {object} usage.ReconciliationItem
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/governance/reconciliations/{id}/resolve [post]
func (s *Server) handleResolveReconciliation(c *gin.Context) {
	if _, ok := s.requireAdmin(c); !ok {
		return
	}
	id := c.Param("id")
	var req ResolveReconciliationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	item, err := s.usageService.ResolveReconciliation(c.Request.Context(), id, req.ResolvedCents, req.Note)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, usage.ErrReconciliationNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

// @Summary 获取允许的模型目录
// @Description 普通用户仅返回已启用且价目未过期的模型；管理员返回全部模型及价目版本状态
// @ID assistantListModels
// @Tags Governance
// @Produce json
// @Success 200 {array} provider.ModelInfo
// @Failure 401 {object} ErrorResponse
// @Router /api/assistant/models [get]
func (s *Server) handleListModels(c *gin.Context) {
	actor, ok := s.authenticate(c)
	if !ok {
		return
	}
	models := s.modelClient.ListCatalog(actor.IsAdmin())
	c.JSON(http.StatusOK, models)
}

// @Summary 更新模型启停状态与价目版本（仅管理员）
// @ID assistantUpdateModel
// @Tags Governance
// @Accept json
// @Produce json
// @Param providerId path string true "提供商ID"
// @Param modelId path string true "模型ID"
// @Param request body UpdateModelRequest true "模型更新内容"
// @Success 200 {object} provider.ModelInfo
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/models/{providerId}/{modelId} [patch]
func (s *Server) handleUpdateModel(c *gin.Context) {
	if _, ok := s.requireAdmin(c); !ok {
		return
	}
	providerID := c.Param("providerId")
	modelID := strings.TrimPrefix(c.Param("modelId"), "/")
	var req UpdateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	updated, err := s.modelClient.UpdateModel(c.Request.Context(), providerID, modelID, provider.ModelUpdate{
		DisplayName:         req.DisplayName,
		PriceVersion:        req.PriceVersion,
		PromptPricePerK:     req.PromptPricePerK,
		CompletionPricePerK: req.CompletionPricePerK,
		EffectiveFrom:       req.EffectiveFrom,
		ExpiresAt:           req.ExpiresAt,
		Enabled:             req.Enabled,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, provider.ErrModelNotAllowed) {
			status = http.StatusNotFound
		}
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// @Summary 列出当前用户有权查看的知识库列表
// @ID assistantListKnowledgeBases
// @Tags Knowledge
// @Produce json
// @Success 200 {array} knowledge.KnowledgeBase
// @Failure 401 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases [get]
func (s *Server) handleListKnowledgeBases(c *gin.Context) {
	actor, ok := s.authenticate(c)
	if !ok {
		return
	}
	bases := s.knowledgeService.ListKnowledgeBases(c.Request.Context(), actor.UserID, actor.IsAdmin())
	c.JSON(http.StatusOK, bases)
}

// @Summary 创建新知识库（仅管理员或讲师）
// @ID assistantCreateKnowledgeBase
// @Tags Knowledge
// @Accept json
// @Produce json
// @Param request body CreateKnowledgeBaseRequest true "创建知识库请求"
// @Success 201 {object} knowledge.KnowledgeBase
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases [post]
func (s *Server) handleCreateKnowledgeBase(c *gin.Context) {
	actor, ok := s.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	var req CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	kb, err := s.knowledgeService.CreateKnowledgeBase(c.Request.Context(), actor.UserID, req.Name, req.Description, req.Visibility)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, kb)
}

// @Summary 获取知识库详情
// @ID assistantGetKnowledgeBase
// @Tags Knowledge
// @Produce json
// @Param id path string true "知识库ID"
// @Success 200 {object} knowledge.KnowledgeBase
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases/{id} [get]
func (s *Server) handleGetKnowledgeBase(c *gin.Context) {
	actor, ok := s.authenticate(c)
	if !ok {
		return
	}
	id := c.Param("id")
	kb, err := s.knowledgeService.GetKnowledgeBase(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Code: http.StatusNotFound, Message: err.Error()})
		return
	}
	if !actor.IsAdmin() && kb.OwnerID != actor.UserID && kb.Visibility != knowledge.VisibilityPublic {
		c.JSON(http.StatusForbidden, ErrorResponse{Code: http.StatusForbidden, Message: ErrForbidden.Error()})
		return
	}
	c.JSON(http.StatusOK, kb)
}

// @Summary 更新知识库元信息与可见范围（仅管理员或归属讲师）
// @ID assistantUpdateKnowledgeBase
// @Tags Knowledge
// @Accept json
// @Produce json
// @Param id path string true "知识库ID"
// @Param request body UpdateKnowledgeBaseRequest true "更新知识库请求"
// @Success 200 {object} knowledge.KnowledgeBase
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases/{id} [patch]
func (s *Server) handleUpdateKnowledgeBase(c *gin.Context) {
	actor, ok := s.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	var req UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	kb, err := s.knowledgeService.UpdateKnowledgeBase(
		c.Request.Context(), id, actor.UserID, actor.IsAdmin(),
		req.ExpectedVersion, req.Name, req.Description, req.Visibility,
	)
	if err != nil {
		c.JSON(mapKnowledgeErrorStatus(err), ErrorResponse{Code: mapKnowledgeErrorStatus(err), Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, kb)
}

// @Summary 列出知识库内的来源列表（仅管理员或归属讲师）
// @ID assistantListKnowledgeSources
// @Tags Knowledge
// @Produce json
// @Param id path string true "知识库ID"
// @Success 200 {array} knowledge.KnowledgeSource
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases/{id}/sources [get]
func (s *Server) handleListKnowledgeSources(c *gin.Context) {
	actor, ok := s.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := s.knowledgeService.CanManageKnowledgeBase(c.Request.Context(), id, actor.UserID, actor.IsAdmin()); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	sources := s.knowledgeService.ListSources(c.Request.Context(), id)
	c.JSON(http.StatusOK, sources)
}

// @Summary 在知识库内登记新来源（仅管理员或归属讲师）
// @ID assistantRegisterKnowledgeSource
// @Tags Knowledge
// @Accept json
// @Produce json
// @Param id path string true "知识库ID"
// @Param request body RegisterSourceRequest true "登记来源请求"
// @Success 201 {object} knowledge.KnowledgeSource
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases/{id}/sources [post]
func (s *Server) handleRegisterKnowledgeSource(c *gin.Context) {
	actor, ok := s.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	var req RegisterSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	src, err := s.knowledgeService.RegisterKnowledgeSource(
		c.Request.Context(), id, actor.UserID, actor.IsAdmin(),
		req.Kind, req.SourceRef, req.Title, req.Content,
	)
	if err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, src)
}

// @Summary 更新知识来源内容并递增版本号（新版本发布前保留原有效版本，仅管理员或归属讲师）
// @ID assistantUpdateKnowledgeSource
// @Tags Knowledge
// @Accept json
// @Produce json
// @Param id path string true "知识库ID"
// @Param sourceId path string true "来源ID"
// @Param request body UpdateSourceRequest true "更新来源请求"
// @Success 200 {object} knowledge.KnowledgeSource
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases/{id}/sources/{sourceId} [patch]
func (s *Server) handleUpdateKnowledgeSource(c *gin.Context) {
	actor, ok := s.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	kbID := c.Param("id")
	sourceID := c.Param("sourceId")
	var req UpdateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	src, err := s.knowledgeService.UpdateKnowledgeSource(
		c.Request.Context(), kbID, sourceID, actor.UserID, actor.IsAdmin(),
		req.Title, req.Content,
	)
	if err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, src)
}

// @Summary 发起知识来源切片与入库索引作业（校验后台日预算，仅管理员或归属讲师）
// @ID assistantStartIndexJob
// @Tags Knowledge
// @Produce json
// @Param id path string true "来源ID"
// @Success 200 {object} knowledge.IndexJob
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 402 {object} knowledge.IndexJob
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/knowledge-sources/{id}/index-jobs [post]
func (s *Server) handleStartIndexJob(c *gin.Context) {
	actor, ok := s.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := s.knowledgeService.CanManageSource(c.Request.Context(), id, actor.UserID, actor.IsAdmin()); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	job, err := s.knowledgeService.StartIndexJob(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, knowledge.ErrBudgetPaused) && job != nil {
			c.JSON(http.StatusPaymentRequired, job)
			return
		}
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

// @Summary 查询知识入库索引作业状态（仅管理员或归属讲师）
// @ID assistantGetIndexJob
// @Tags Knowledge
// @Produce json
// @Param id path string true "作业ID"
// @Success 200 {object} knowledge.IndexJob
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/index-jobs/{id} [get]
func (s *Server) handleGetIndexJob(c *gin.Context) {
	actor, ok := s.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := s.knowledgeService.CanManageJob(c.Request.Context(), id, actor.UserID, actor.IsAdmin()); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	job, err := s.knowledgeService.GetIndexJob(c.Request.Context(), id)
	if err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

// @Summary 发布知识来源使其可被检索（仅管理员或归属讲师）
// @ID assistantPublishKnowledgeSource
// @Tags Knowledge
// @Accept json
// @Produce json
// @Param id path string true "来源ID"
// @Param request body PublishSourceRequest true "发布版本请求"
// @Success 200 {object} StandardResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/assistant/knowledge-sources/{id}/publish [post]
func (s *Server) handlePublishKnowledgeSource(c *gin.Context) {
	actor, ok := s.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := s.knowledgeService.CanManageSource(c.Request.Context(), id, actor.UserID, actor.IsAdmin()); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	var req PublishSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	if err := s.knowledgeService.PublishKnowledgeSource(c.Request.Context(), id, req.ExpectedVersion); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StandardResponse{Code: http.StatusOK, Message: "source published successfully"})
}

// @Summary 立即撤回知识来源并清理派生切片（仅管理员或归属讲师）
// @ID assistantRevokeKnowledgeSource
// @Tags Knowledge
// @Produce json
// @Param id path string true "来源ID"
// @Success 200 {object} StandardResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/knowledge-sources/{id}/revoke [post]
func (s *Server) handleRevokeKnowledgeSource(c *gin.Context) {
	actor, ok := s.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := s.knowledgeService.CanManageSource(c.Request.Context(), id, actor.UserID, actor.IsAdmin()); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	if err := s.knowledgeService.RevokeKnowledgeSource(c.Request.Context(), id); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StandardResponse{Code: http.StatusOK, Message: "source revoked successfully"})
}

// @Summary 列出可见的 Agent 实例
// @Description 普通用户仅返回已发布实例且隐藏内部 systemPrompt；管理员返回全部（含草稿与停用）
// @ID assistantListAgents
// @Tags Agents
// @Produce json
// @Success 200 {array} agent.Configuration
// @Failure 401 {object} ErrorResponse
// @Router /api/assistant/agents [get]
func (s *Server) handleListAgents(c *gin.Context) {
	actor, ok := s.authenticate(c)
	if !ok {
		return
	}
	agents := s.agentService.ListAgents(c.Request.Context(), actor.IsAdmin())
	c.JSON(http.StatusOK, agents)
}

// @Summary 根据标识获取 Agent 实例详情
// @ID assistantGetAgentBySlug
// @Tags Agents
// @Produce json
// @Param slug path string true "Agent标识(如 customer_support 或 course_tutor)"
// @Success 200 {object} agent.Configuration
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/agents/{slug} [get]
func (s *Server) handleGetAgentBySlug(c *gin.Context) {
	actor, ok := s.authenticate(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	cfg, err := s.agentService.GetAgentBySlug(c.Request.Context(), slug, actor.IsAdmin())
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
// @Success 201 {object} agent.Configuration
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/assistant/agents [post]
func (s *Server) handleCreateAgent(c *gin.Context) {
	if _, ok := s.requireAdmin(c); !ok {
		return
	}
	var req CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	cfg, err := s.agentService.CreateAgent(
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
// @Success 200 {object} agent.Configuration
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/assistant/agents/{id} [patch]
func (s *Server) handleUpdateAgent(c *gin.Context) {
	if _, ok := s.requireAdmin(c); !ok {
		return
	}
	id := c.Param("id")
	var req UpdateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	cfg, err := s.agentService.UpdateAgent(c.Request.Context(), id, agent.UpdateAgentInput{
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
// @Success 200 {object} agent.Configuration
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/agents/{id}/publish [post]
func (s *Server) handlePublishAgent(c *gin.Context) {
	if _, ok := s.requireAdmin(c); !ok {
		return
	}
	id := c.Param("id")
	cfg, err := s.agentService.PublishAgent(c.Request.Context(), id)
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
// @Success 200 {object} agent.Configuration
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/agents/{id}/disable [post]
func (s *Server) handleDisableAgent(c *gin.Context) {
	if _, ok := s.requireAdmin(c); !ok {
		return
	}
	id := c.Param("id")
	var req DisableAgentRequest
	_ = c.ShouldBindJSON(&req)
	cfg, err := s.agentService.DisableAgent(c.Request.Context(), id, req.Reason)
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
// @Success 200 {array} agent.Session
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/assistant/sessions [get]
func (s *Server) handleListSessions(c *gin.Context) {
	actor, ok := s.requireVerifiedLearner(c)
	if !ok {
		return
	}
	sessions := s.agentService.ListSessions(c.Request.Context(), actor.UserID)
	c.JSON(http.StatusOK, sessions)
}

// @Summary 创建问答会话（需已验证邮箱）
// @ID assistantCreateSession
// @Tags Chat
// @Accept json
// @Produce json
// @Param request body CreateSessionRequest true "创建会话请求"
// @Success 201 {object} agent.Session
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/sessions [post]
func (s *Server) handleCreateSession(c *gin.Context) {
	actor, ok := s.requireVerifiedLearner(c)
	if !ok {
		return
	}
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	sess, err := s.agentService.CreateSession(c.Request.Context(), req.AgentSlug, actor.UserID, req.CourseID, req.Title)
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
// @Success 200 {object} agent.Session
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/sessions/{id} [get]
func (s *Server) handleGetSession(c *gin.Context) {
	actor, ok := s.requireVerifiedLearner(c)
	if !ok {
		return
	}
	id := c.Param("id")
	sess, err := s.agentService.GetSession(c.Request.Context(), id, actor.UserID)
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
// @Success 200 {array} agent.Message
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/sessions/{id}/messages [get]
func (s *Server) handleListMessages(c *gin.Context) {
	actor, ok := s.requireVerifiedLearner(c)
	if !ok {
		return
	}
	id := c.Param("id")
	msgs, err := s.agentService.ListMessages(c.Request.Context(), id, actor.UserID)
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
// @Success 200 {object} agent.Message
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 402 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/assistant/sessions/{id}/ask [post]
func (s *Server) handleAskQuestion(c *gin.Context) {
	actor, ok := s.requireVerifiedLearner(c)
	if !ok {
		return
	}
	id := c.Param("id")
	var req AskQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	msg, err := s.agentService.AskWithOptions(c.Request.Context(), id, actor.UserID, req.Question, req.IdempotencyKey)
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
// @Success 200 {object} agent.Run
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/sessions/{id}/runs/{runId}/cancel [post]
func (s *Server) handleCancelRun(c *gin.Context) {
	actor, ok := s.requireVerifiedLearner(c)
	if !ok {
		return
	}
	sessionID := c.Param("id")
	runID := c.Param("runId")
	run, err := s.agentService.CancelRun(c.Request.Context(), sessionID, runID, actor.UserID)
	if err != nil {
		status := mapAgentErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, run)
}

// @Summary 内部接口：课程图文知识同步（按 knowledgeBaseId+courseId 去重或升版本，严格校验 X-Service-Token）
// @ID assistantInternalSyncCourseKnowledge
// @Tags Internal
// @Accept json
// @Produce json
// @Param X-Service-Token header string true "服务间认证令牌"
// @Param request body SyncCourseKnowledgeRequest true "同步课程内容"
// @Success 200 {object} StandardResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 402 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /internal/assistant/knowledge/sync-course [post]
func (s *Server) handleSyncCourseKnowledge(c *gin.Context) {
	if !s.verifyServiceToken(c) {
		return
	}
	var req SyncCourseKnowledgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	src, job, err := s.knowledgeService.SyncCourseKnowledge(
		c.Request.Context(), req.KnowledgeBaseID, req.CourseID, req.Title, req.Content,
	)
	if err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StandardResponse{
		Code:    http.StatusOK,
		Message: "course knowledge synced",
		Data: gin.H{
			"source": src,
			"job":    job,
		},
	})
}

func mapKnowledgeErrorStatus(err error) int {
	switch {
	case errors.Is(err, knowledge.ErrKnowledgeBaseNotFound),
		errors.Is(err, knowledge.ErrKnowledgeSourceNotFound),
		errors.Is(err, knowledge.ErrIndexJobNotFound):
		return http.StatusNotFound
	case errors.Is(err, knowledge.ErrUnauthorizedSourceScope):
		return http.StatusForbidden
	case errors.Is(err, knowledge.ErrVersionMismatch),
		errors.Is(err, knowledge.ErrSourceRevoked):
		return http.StatusConflict
	case errors.Is(err, knowledge.ErrBudgetPaused):
		return http.StatusPaymentRequired
	default:
		return http.StatusBadRequest
	}
}

func mapAgentErrorStatus(err error) int {
	switch {
	case errors.Is(err, agent.ErrAgentNotFound),
		errors.Is(err, agent.ErrSessionNotFound),
		errors.Is(err, agent.ErrRunNotFound):
		return http.StatusNotFound
	case errors.Is(err, agent.ErrUnauthorizedAccess),
		errors.Is(err, agent.ErrAgentDisabled),
		errors.Is(err, agent.ErrCitationAccessRevoked):
		return http.StatusForbidden
	case errors.Is(err, agent.ErrSlugAlreadyExists),
		errors.Is(err, agent.ErrVersionMismatch),
		errors.Is(err, agent.ErrIdempotencyConflict),
		errors.Is(err, agent.ErrRunInProgress):
		return http.StatusConflict
	case errors.Is(err, usage.ErrRateLimitExceeded),
		errors.Is(err, usage.ErrUserConcurrencyLimit),
		errors.Is(err, usage.ErrPlatformConcurrency),
		errors.Is(err, usage.ErrDailyLimitExceeded):
		return http.StatusTooManyRequests
	case errors.Is(err, usage.ErrUserBudgetExceeded),
		errors.Is(err, usage.ErrPlatformBudget),
		errors.Is(err, usage.ErrPlatformMonthlyBudget),
		errors.Is(err, usage.ErrSingleRequestBudgetExceeded):
		return http.StatusPaymentRequired
	case errors.Is(err, usage.ErrAIPaused),
		errors.Is(err, provider.ErrProviderNotConfigured),
		errors.Is(err, provider.ErrSearchNotConfigured),
		errors.Is(err, provider.ErrModelPriceExpired):
		return http.StatusServiceUnavailable
	case errors.Is(err, provider.ErrSearchFailed),
		errors.Is(err, provider.ErrSearchAuthFailed),
		errors.Is(err, provider.ErrSearchTimeout),
		errors.Is(err, provider.ErrModelGenerationFailed):
		return http.StatusBadGateway
	default:
		return http.StatusBadRequest
	}
}
