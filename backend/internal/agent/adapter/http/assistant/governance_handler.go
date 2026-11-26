package assistant

import (
	"errors"
	"net/http"
	"strings"

	providerdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/provider"
	usagedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/usage"
	"github.com/gin-gonic/gin"
)

// @Summary 获取当前用户的 AI 问答额度与用量治理概况
// @Description 返回本人今日成功次数与预算占用；管理员额外返回平台日月预算、后台子额度、未知费用与 80% 告警标志
// @ID assistantGetGovernanceSummary
// @Tags Governance
// @Produce json
// @Success 200 {object} usagedomain.UsageSummary
// @Failure 401 {object} ErrorResponse
// @Router /api/assistant/governance/summary [get]
func (m *Handler) handleGetGovernanceSummary(c *gin.Context) {
	actor, ok := m.authenticate(c)
	if !ok {
		return
	}
	summary := m.usageService.GetSummary(c.Request.Context(), actor.UserID, actor.IsAdmin())
	c.JSON(http.StatusOK, summary)
}

// @Summary 更新平台与账号 AI 预算限额配置（仅管理员）
// @ID assistantUpdateBudgetConfig
// @Tags Governance
// @Accept json
// @Produce json
// @Param request body UpdateBudgetConfigRequest true "预算配置更新请求"
// @Success 200 {object} usagedomain.BudgetConfig
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/assistant/governance/budget [patch]
func (m *Handler) handleUpdateBudgetConfig(c *gin.Context) {
	if _, ok := m.requireAdmin(c); !ok {
		return
	}
	var req UpdateBudgetConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	cfg, err := m.usageService.UpdateConfig(c.Request.Context(), usagedomain.BudgetConfigUpdate{
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
// @Success 200 {object} usagedomain.BudgetConfig
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/assistant/governance/pause [post]
func (m *Handler) handlePauseAI(c *gin.Context) {
	if _, ok := m.requireAdmin(c); !ok {
		return
	}
	var req PauseAIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	cfg, err := m.usageService.SetPaused(c.Request.Context(), req.Paused, req.Reason)
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
// @Success 200 {array} usagedomain.ReconciliationItem
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/assistant/governance/reconciliations [get]
func (m *Handler) handleListReconciliations(c *gin.Context) {
	if _, ok := m.requireAdmin(c); !ok {
		return
	}
	items := m.usageService.ListReconciliations(c.Request.Context())
	c.JSON(http.StatusOK, items)
}

// @Summary 人工核对并结清未知费用条目（仅管理员）
// @ID assistantResolveReconciliation
// @Tags Governance
// @Accept json
// @Produce json
// @Param id path string true "未知费用核对ID"
// @Param request body ResolveReconciliationRequest true "核对结果"
// @Success 200 {object} usagedomain.ReconciliationItem
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/governance/reconciliations/{id}/resolve [post]
func (m *Handler) handleResolveReconciliation(c *gin.Context) {
	if _, ok := m.requireAdmin(c); !ok {
		return
	}
	id := c.Param("id")
	var req ResolveReconciliationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	item, err := m.usageService.ResolveReconciliation(c.Request.Context(), id, req.ResolvedCents, req.Note)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, usagedomain.ErrReconciliationNotFound) {
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
// @Success 200 {array} providerdomain.ModelInfo
// @Failure 401 {object} ErrorResponse
// @Router /api/assistant/models [get]
func (m *Handler) handleListModels(c *gin.Context) {
	actor, ok := m.authenticate(c)
	if !ok {
		return
	}
	models := m.modelClient.ListCatalog(actor.IsAdmin())
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
// @Success 200 {object} providerdomain.ModelInfo
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/models/{providerId}/{modelId} [patch]
func (m *Handler) handleUpdateModel(c *gin.Context) {
	if _, ok := m.requireAdmin(c); !ok {
		return
	}
	providerID := c.Param("providerId")
	modelID := strings.TrimPrefix(c.Param("modelId"), "/")
	var req UpdateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	updated, err := m.modelClient.UpdateModel(c.Request.Context(), providerID, modelID, providerdomain.ModelUpdate{
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
		if errors.Is(err, providerdomain.ErrModelNotAllowed) {
			status = http.StatusNotFound
		}
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}
