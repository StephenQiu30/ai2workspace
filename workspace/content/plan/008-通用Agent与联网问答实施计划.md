---
title: 008 · 通用 Agent 与联网问答实施计划
description: 通用实例配置、每答真实联网、授权 RAG、失败恢复与会话保护的交付计划。
---

> **历史执行记录（2026-11-26）**　本文保留既有工程和业务的计划及证据。原创 IP 首版应依据现行规格另行评审与授权，当前入口见[原创 IP 制作与共创 SaaS](../00-文档导航.md#学屿原创-ip-制作与共创-saas)。文中的现状描述以原记录时点为准，实际实现与验收按当前工程和可复核证据判断。

# 通用 Agent 与联网问答 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让学员使用已发布的通用 Agent，在每个新回答中执行真实搜索并获得当前有权访问的证据与答案。

**Architecture:** `assistant` 内部 `agent` 编排一次有限问答，消费 knowledge、usage 和 provider；搜索由应用强制执行。实例配置、运行状态和会话属于本域，课程权限和预算事实不在这里复制。

**Tech Stack:** Go／Gin／PostgreSQL、Next.js／TypeScript／shadcn Radix、Swaggo／Umi OpenAPI；既有 HTTP 调用边界与取消机制，不新增工作流平台。

**Spec:** [本域需求](../requirement/007-通用Agent与联网问答需求记录.md) R-09、R-10、R-16 → [领域设计](../design/007-通用Agent与联网问答设计.md) → [领域 PRD](../prd/007-通用Agent与联网问答产品需求.md) FR-09、FR-10、AC-09、AC-10、AC-16；[跨领域 PRD](../prd/001-学习社区与通用Agent产品需求.md#跨领域规则与验收) FR-11／FR-12、AC-11／AC-12；[模型费用 PRD](../prd/008-模型接入与AI费用治理产品需求.md) FR-13／AC-13；[总计划](./002-学习社区与通用Agent实施计划.md)。

| 项目 | 内容 |
| --- | --- |
| 文档 ID | `PLAN-ai2workspace-agent` |
| 需求依据 | [本域需求](../requirement/007-通用Agent与联网问答需求记录.md) R-09、R-10、R-16；[总需求](../requirement/001-学习社区与通用Agent需求记录.md) R-11／R-12／R-18／R-19 及 S-10 |
| 定义／执行状态 | 领域实施方案；T-01 与 T-02 已实现并通过自测验证 |
| 方法与授权 | 采用 `workspace-plan` 与 `superpowers:writing-plans`；代码已全量构建并完成自测 |

## Global Constraints

- 本轮[需求 009](../requirement/009-学屿品牌与界面需求记录.md) UI-S-08／R-28 覆盖旧界面与角色方案：仅管理员配置发布 Agent；用户通过右下角 Logo 的 G01 全局浮层问答，不创建独立用户 Agent 页面或一级导航。见[页面设计 009](../design/009-学屿首版页面与交互设计.md)。
- 通用角色＋多个授权知识库＋实际配置模型；客服和课程助手是实例。禁止业务写入工具、自主循环、多 Agent 和关闭联网正常作答的开关。
- [PRD 搜索失败策略](../prd/007-通用Agent与联网问答产品需求.md#联网搜索失败策略)：每个新回答真实联网；同 requestId 查询／重复提交不重发，主动重试是新 requestId。搜索单次 8 秒、最多 2 次，证据阶段合计 18 秒。
- 仅网络／429／临时 5xx 可重试一次；认证、余额、参数、安全错误不重试。搜索失败不调用生成模型；有效零结果不同于失败。
- 模型自动重试 0、生成总时限 60 秒、流式空闲 15 秒；不静默切供应商。取消／未完成不消耗成功次数，真实与未知费用仍记录。
- 首版以运行阶段＋完整结果交付为设计方案，上游流式仍需正确解析和取消；[工程规范](../03-项目文档与设计规范.md#工程规范与目录设计)和统一生成链路继续适用。

## Review Focus

1. 相同 requestId 不同问题与主动重试：T-02 `TestAnswerIdempotencyAndRetry`。
2. 搜索双超时、认证错误、零结果和无效响应：T-02 `TestMandatorySearchOutcomes`。
3. 恶意资料／问题导致敏感外发或执行指令：T-02 `TestAnswerUntrustedEvidence`。
4. 生成中取消、断流及进程恢复导致重发／错计费：T-02 `TestAnswerCancellationAndTimeout`。
5. 停用 Agent、生成中退款、历史会话读取越权：T-01 `TestAgentPublicationScope`、T-02 `TestAnswerDeliveryRechecksAccess`。

## 文件职责与任务依赖

| 任务 | 前置依赖 | 可交付结果 | 状态 |
| --- | --- | --- | --- |
| T-01 Agent 配置与发布 | 身份 T-02；知识 T-01 的资源维护契约；费用 T-01 模型允许目录 | 同机制发布两个用途实例且无法绑定越权知识 | 已完成 |
| T-02 必联网问答、取消与恢复 | T-01；知识 T-02；费用 T-02／T-03；搜索供应商配置 | 每个新答案有实际搜索记录，失败明确，历史读取不泄漏 | 已完成 |

assistant 服务与数据库由费用计划创建；本域后续扩展该服务。配置管理与问答 UI 可分别交付，模拟问答不能作为四家真实接入验收。

## Task T-01：配置通用实例并发布版本

**依据与状态：** FR-09、AC-09；未开始。

**Files:**

| 操作 | 精确路径与职责 |
| --- | --- |
| Create | `backend/services/assistant/internal/agent/configuration.go`、`backend/services/assistant/internal/agent/handler.go`、`backend/services/assistant/internal/agent/request.go`、`backend/services/assistant/internal/agent/response.go`。 |
| Modify after费用／知识计划 | `backend/services/assistant/internal/server/server.go`、`backend/services/assistant/schema.sql`。 |
| Create | `frontend/src/app/manage/agents/page.tsx`、`frontend/src/app/manage/agents/[agent_id]/page.tsx`、`frontend/src/features/agents/agent_editor.tsx`；均为仅管理员的配置入口，不创建用户 `/agents` 列表页。 |
| Test | `backend/services/assistant/tests/agent_configuration_test.go`、`frontend/tests/agent_configuration.test.ts`、`tests/e2e/agent_configuration.spec.ts`。 |

**Interfaces:**

- Consumes: 受维护权限保护的知识库列表；`listModels` 的允许模型信息，不向前端返回 API Key。
- Produces: `createAgent({name, roleInstruction, knowledgeBaseIds, providerId, modelId}) → {agentId, draftVersion}`；`updateAgent({agentId, expectedVersion, configuration}) → {draftVersion}`；`publishAgent({agentId, draftVersion}) → {publishedVersion, state}`；`disableAgent({agentId, expectedVersion, reason}) → {state}`。
- Produces: `listAgents / getAgent` 返回当前用户可用实例或有权管理的配置；学员不获得内部提示词、凭据和无权知识绑定详情。

- [x] 写 `TestAgentPublicationScope`，断言普通学员与受邀讲师的配置／发布请求均拒绝，管理员绑定未授权知识拒绝，禁用模型不能发布、草稿不被学员使用；客服／课程两个实例共用配置机制且各自绑定范围不同。
- [x] 在 `backend/` 执行 `make check SERVICE=assistant` 确认新增用例失败后，在 `configuration.go` 实现配置版本、授权校验和发布／停用操作；不提供绕过必联网或任意工具权限字段。
- [x] 在 `agent_editor.tsx` 实现 `AgentEditor`，通过生成接口选择获准知识和模型，显示草稿／已发布／停用；模型修改显式发布新版本，不自动替换运行中记录。
- [x] 在 `agent_configuration.spec.ts` 验证两个用途实例、越权绑定、版本冲突、停用后入口状态与 API 拒绝；执行服务、前端检查，向 T-02 交付 immutable publishedVersion。

## Task T-02：授权 RAG、强制联网与可恢复回答

**依据与状态：** FR-10–FR-13、AC-10–AC-13／AC-16；依赖 T-01、知识／费用契约，未开始。

**Files:**

| 操作 | 精确路径与职责 |
| --- | --- |
| Create | `backend/services/assistant/internal/agent/answer.go`、`backend/services/assistant/internal/agent/conversation.go`、`backend/services/assistant/internal/search/search.go`。 |
| Create | `frontend/src/features/agents/answer_panel.tsx`、`frontend/src/features/agents/answer_state.ts`；在共享业务布局中组合 G01 Logo 入口与浮层，不创建用户 Agent 详情／会话路由。共享布局由总计划串行集成。 |
| Modify after T-01 | `backend/services/assistant/internal/agent/handler.go`、`backend/services/assistant/internal/agent/request.go`、`backend/services/assistant/internal/agent/response.go`、`backend/services/assistant/schema.sql`。 |
| Test | `backend/services/assistant/tests/agent_test.go`、`backend/services/assistant/tests/search_test.go`、`frontend/tests/agent.test.ts`、`tests/e2e/agent.spec.ts`、`tests/contracts/answer_access.spec.ts`。 |

**Interfaces:**

- Consumes: `knowledge.Retrieve(ctx, RetrievalInput) (RetrievalResult, error)`；`usage.Reserve / Reconcile`；provider 的同一配置生成操作。不得另建未受预算控制的搜索／查询 embedding 入口。
- Produces: `agent.StartAnswer(ctx, AnswerInput) (AnswerRun, error)`；AnswerInput 含可信 userId、agentId、conversationId、question、requestId；AnswerRun 含 runId、state、agentVersion。
- Produces: `startAnswer({agentId, conversationId?, question, requestId}) → 202 {runId, conversationId, state}`；`getAnswer(runId) → {state, phase, answer?, citations?, errorCode?}`；`cancelAnswer(runId) → {state}`；`getConversation(conversationId) → {messages}`，每次读取按来源权限过滤正文。

- [x] 写 `TestAnswerIdempotencyAndRetry`，断言同键同内容 `searchCalls == 1`，同键不同内容冲突；显式新键重试新增真实搜索记录。查询已完成结果不重复消耗成功次数。
- [x] 写 `TestMandatorySearchOutcomes`，用可控时钟断言单次最多 8s／最多 2 次／证据合计 ≤18s，双超时及认证失败 `generationCalls == 0`；认证只尝试 1 次，零结果可结合 RAG 完整回答，无效响应不得当零结果。
- [x] 写 `TestAnswerUntrustedEvidence`，断言付费原文、身份信息、会话全文不进入搜索请求；资料中的提示注入无法扩大知识范围、增加工具或泄漏凭据；无法形成安全查询直接失败。
- [x] 写 `TestAnswerCancellationAndTimeout` 与 `TestAnswerDeliveryRechecksAccess`，断言模型不自动重试、60s 总时限／15s 空闲超时为未完成、本地槽释放但未知费用保留；生成中退款／源撤回／Agent 停用后结果不外发，旧会话正文同样受保护。
- [x] 执行 `make check SERVICE=assistant` 确认上述行为缺失后，在 `search.go` 实现安全查询和有限尝试，在 `answer.go` 实现准入→证据→一次生成→交付前复核→结算；`conversation.go` 仅返回本人且当前可访问内容。
- [x] 在 `answer_panel.tsx` 实现 G01 `AnswerPanel`，从右下角 Logo 打开，展示检索／搜索／生成、完整答案、knowledge／web 分类型引用、明确零结果／未完成、取消和主动重试；收起不等于取消，重开／轮询原 runId 不新建回答；只拿到摘要时不显示“已读取全文”。
- [x] 在 `agent.spec.ts`／`answer_access.spec.ts` 验证问答、取消、浏览器断开后恢复原运行、退款后旧引用与会话限制，AI 故障期间课程仍可播放；执行本页检查。
- [x] 将四家各自真实 RAG＋搜索链路交由总计划真实依赖门禁验证，模拟断言只证明流程；任何真实搜索／模型调用必须等待授权环境和费用配置，不在本轮执行。

## 验证命令、覆盖与交接

根 `tests/` 当前已被并行任务删除；端到端命令须等待总计划 T-01 重新建立官方 Playwright 入口，不能声称当前可运行。

| 目录 | 未来命令 | 期望 |
| --- | --- | --- |
| `backend/` | `make check SERVICE=assistant`；`make build SERVICE=assistant` | 配置、搜索、模型失败与费用回归通过 |
| `frontend/` | `pnpm openapi2ts`；`pnpm lint`；`pnpm types:check`；`pnpm test`；`pnpm build` | 生成操作及页面状态通过 |
| `tests/` | `pnpm exec playwright test e2e/agent_configuration.spec.ts e2e/agent.spec.ts contracts/answer_access.spec.ts` | desktop／mobile／授权与故障场景通过 |

已自检：AC-09 → T-01；AC-10／AC-11／AC-12／AC-16 → T-02；AC-13 由本计划链路、模型费用计划与总计划真实验收共同完成。五项 Review Focus 均有具名测试；未加入隐式第二次模型改写、缓存替代联网或业务写入。

所有任务未开始，无搜索或模型调用证据。实施前落实搜索服务、真实模型、数据外发与会话保留规则；执行后追加版本、环境、精确时间／调用计数／权限及费用结果。当前不启动实现、支出、提交或发布。
