package assistant

import (
	"errors"
	"net/http"

	knowledgedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/knowledge"
	"github.com/gin-gonic/gin"
)

// @Summary 列出当前用户有权查看的知识库列表
// @ID assistantListKnowledgeBases
// @Tags Knowledge
// @Produce json
// @Success 200 {array} knowledgedomain.KnowledgeBase
// @Failure 401 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases [get]
func (m *Handler) handleListKnowledgeBases(c *gin.Context) {
	actor, ok := m.authenticate(c)
	if !ok {
		return
	}
	bases := m.knowledgeService.ListKnowledgeBases(c.Request.Context(), actor.UserID, actor.IsAdmin())
	c.JSON(http.StatusOK, bases)
}

// @Summary 创建新知识库（仅管理员或讲师）
// @ID assistantCreateKnowledgeBase
// @Tags Knowledge
// @Accept json
// @Produce json
// @Param request body CreateKnowledgeBaseRequest true "创建知识库请求"
// @Success 201 {object} knowledgedomain.KnowledgeBase
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases [post]
func (m *Handler) handleCreateKnowledgeBase(c *gin.Context) {
	actor, ok := m.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	var req CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	kb, err := m.knowledgeService.CreateKnowledgeBase(c.Request.Context(), actor.UserID, req.Name, req.Description, req.Visibility)
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
// @Success 200 {object} knowledgedomain.KnowledgeBase
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases/{id} [get]
func (m *Handler) handleGetKnowledgeBase(c *gin.Context) {
	actor, ok := m.authenticate(c)
	if !ok {
		return
	}
	id := c.Param("id")
	kb, err := m.knowledgeService.GetKnowledgeBase(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Code: http.StatusNotFound, Message: err.Error()})
		return
	}
	if !actor.IsAdmin() && kb.OwnerID != actor.UserID && kb.Visibility != knowledgedomain.VisibilityPublic {
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
// @Success 200 {object} knowledgedomain.KnowledgeBase
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases/{id} [patch]
func (m *Handler) handleUpdateKnowledgeBase(c *gin.Context) {
	actor, ok := m.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	var req UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	kb, err := m.knowledgeService.UpdateKnowledgeBase(
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
// @Success 200 {array} knowledgedomain.KnowledgeSource
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases/{id}/sources [get]
func (m *Handler) handleListKnowledgeSources(c *gin.Context) {
	actor, ok := m.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := m.knowledgeService.CanManageKnowledgeBase(c.Request.Context(), id, actor.UserID, actor.IsAdmin()); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	sources := m.knowledgeService.ListSources(c.Request.Context(), id)
	c.JSON(http.StatusOK, sources)
}

// @Summary 在知识库内登记新来源（仅管理员或归属讲师）
// @ID assistantRegisterKnowledgeSource
// @Tags Knowledge
// @Accept json
// @Produce json
// @Param id path string true "知识库ID"
// @Param request body RegisterSourceRequest true "登记来源请求"
// @Success 201 {object} knowledgedomain.KnowledgeSource
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases/{id}/sources [post]
func (m *Handler) handleRegisterKnowledgeSource(c *gin.Context) {
	actor, ok := m.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	var req RegisterSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	src, err := m.knowledgeService.RegisterKnowledgeSource(
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
// @Success 200 {object} knowledgedomain.KnowledgeSource
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/knowledge-bases/{id}/sources/{sourceId} [patch]
func (m *Handler) handleUpdateKnowledgeSource(c *gin.Context) {
	actor, ok := m.requireKnowledgeMaintainer(c)
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
	src, err := m.knowledgeService.UpdateKnowledgeSource(
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
// @Success 200 {object} knowledgedomain.IndexJob
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 402 {object} knowledgedomain.IndexJob
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/knowledge-sources/{id}/index-jobs [post]
func (m *Handler) handleStartIndexJob(c *gin.Context) {
	actor, ok := m.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := m.knowledgeService.CanManageSource(c.Request.Context(), id, actor.UserID, actor.IsAdmin()); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	job, err := m.knowledgeService.StartIndexJob(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, knowledgedomain.ErrBudgetPaused) && job != nil {
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
// @Success 200 {object} knowledgedomain.IndexJob
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/assistant/index-jobs/{id} [get]
func (m *Handler) handleGetIndexJob(c *gin.Context) {
	actor, ok := m.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := m.knowledgeService.CanManageJob(c.Request.Context(), id, actor.UserID, actor.IsAdmin()); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	job, err := m.knowledgeService.GetIndexJob(c.Request.Context(), id)
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
func (m *Handler) handlePublishKnowledgeSource(c *gin.Context) {
	actor, ok := m.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := m.knowledgeService.CanManageSource(c.Request.Context(), id, actor.UserID, actor.IsAdmin()); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	var req PublishSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	if err := m.knowledgeService.PublishKnowledgeSource(c.Request.Context(), id, req.ExpectedVersion); err != nil {
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
func (m *Handler) handleRevokeKnowledgeSource(c *gin.Context) {
	actor, ok := m.requireKnowledgeMaintainer(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := m.knowledgeService.CanManageSource(c.Request.Context(), id, actor.UserID, actor.IsAdmin()); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	if err := m.knowledgeService.RevokeKnowledgeSource(c.Request.Context(), id); err != nil {
		status := mapKnowledgeErrorStatus(err)
		c.JSON(status, ErrorResponse{Code: status, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, StandardResponse{Code: http.StatusOK, Message: "source revoked successfully"})
}

func mapKnowledgeErrorStatus(err error) int {
	switch {
	case errors.Is(err, knowledgedomain.ErrKnowledgeBaseNotFound),
		errors.Is(err, knowledgedomain.ErrKnowledgeSourceNotFound),
		errors.Is(err, knowledgedomain.ErrIndexJobNotFound):
		return http.StatusNotFound
	case errors.Is(err, knowledgedomain.ErrUnauthorizedSourceScope):
		return http.StatusForbidden
	case errors.Is(err, knowledgedomain.ErrVersionMismatch),
		errors.Is(err, knowledgedomain.ErrSourceRevoked):
		return http.StatusConflict
	case errors.Is(err, knowledgedomain.ErrBudgetPaused):
		return http.StatusPaymentRequired
	default:
		return http.StatusBadRequest
	}
}
