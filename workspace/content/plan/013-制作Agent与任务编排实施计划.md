---
title: 013 · 制作Agent与任务编排实施计划
description: 在制作页面执行编剧、分镜和一致性校验，保留有证据的候选、可恢复的运行记录与可核对的费用。
---

> **执行者须知**　评审并获准执行后，选用 `superpowers:subagent-driven-development` 或 `superpowers:executing-plans`，按任务逐项验证。方框记录执行进度。

**Goal**　在制作页面执行编剧、分镜和一致性校验，保留有证据的候选、可恢复的运行记录与可核对的费用。

**Architecture**　`agent` 持有运行、步骤与费用账本。助手模块执行 Agent 步骤，通过 AI 网关调用模型，经 Redis/SSE 发出进度。按任务恢复需求使用 Temporal。`creation` 独立执行人工采纳。

**Tech Stack**　Go / PostgreSQL、Temporal Go / TS SDK、Vercel AI SDK、独立 AI 网关、Redis / SSE、Next.js。

**Spec**　[需求](../requirement/013-制作Agent与任务编排需求记录.md) · [专题设计](../design/014-制作Agent与任务编排设计.md) · [专题 PRD](../prd/012-制作Agent与任务编排产品需求.md) · [系统设计与联合验收](../design/018-原创IP首版系统设计与验收总览.md) · [项目总体设计](../../01-项目总体设计.md#7-选型与验证)

| 计划 ID | 专题 | 定义与执行状态 |
| --- | --- | --- |
| `PLAN-learnis-production-agent` | 学屿（`ai2workspace`）／制作Agent与任务编排 | 计划草案，任务未开始 |

## Global Constraints

- 固定为编剧、分镜、一致性校验三步。媒体生成遵守同一准入、候选与费用规则。渲染产物规则由成片计划维护。正式稿、发布和交易只由对应用户接口操作。
- 媒体生成供应商和 AI 网关产品未定，先使用接口桩。未知成本依据已确认的供应商事件追查。
- 文中新增的业务代码、测试与页面路径是规划目标。现有 `frontend/src/app/manage/ai-usage/page.tsx` 与旧助手的账务行为作为回归依据。后端 REST 契约从 Go DTO 和 Swaggo 生成，再更新 Umi 客户端。
- 测试命令在目标工程建立后使用。真实付费、外部生产实例、提交和发布分别按实际授权执行。

## Review Focus

- TestRunAdmission：并发请求只创建一次运行和预算预占。
- TestUnknownAttempt：超时或接收失败时保留供应商原请求，避免重复费用。
- TestAgentCandidateBoundary：含状态变更指令的资料或候选无法越权。
- TestStepRecovery / TestRunEventsAccess：失败步骤重试、迟到结果和 SSE 断线均受版本及权限约束。
- TestUsageReconciliation：重复扣费回报或取消后的迟到用量只结算一次。

## 前置依赖与文件职责

编排与费用可以用模拟依赖验证。候选联调需 [011 T-01/T-02](./011-剧本分镜与成片制作实施计划.md)，真实上下文需 [012 T-02/T-03](./012-制作知识库与RAG实施计划.md)。真实模型调用在供应商、额度与费用核对责任确认后执行。

| 任务 | 前置依赖 | 主要交付 |
| --- | --- | --- |
| T-01 | 010 T-01，候选联调接 011 T-02 | 登记可恢复运行并预占预算 |
| T-02 | T-01，011 T-01、T-02，012 T-02、T-03 | 运行三步制作 Agent 并人工采纳 |
| T-03 | T-01、T-02 | 提供任务状态与步骤恢复 |
| T-04 | T-01、T-03 | 核对用量与取消后的费用 |

### Task T-01 登记可恢复运行并预占预算

**依据与依赖**　R-02 / FR-02 / AC-02。R-03 / FR-03 / AC-03。NFR-02 / AC-12。前置为 010 T-01。候选联调接 011 T-02。对照本专题需求与 PRD。**状态**　未开始。

**Files**

新增

```text
backend/internal/agent/application/agent/production_run.go
backend/internal/agent/application/usage/production_budget.go
backend/internal/agent/application/agent/production_workflow.go
```

在 `backend/schema.sql` 更新对应业务表。

**Test**

```text
backend/tests/agent/production_agent_run_test.go
```

**Interfaces**　StartProductionRun(work_id, input_version_id, step, idempotency_key) → run_id 与准入状态。RetryStep(run_id, step_id) → 已确认可重试的 attempt_id。

- [ ] TestRunAdmission：并发重复点击返回同一 run_id、一次预算预占。额度不足的请求不会到达网关。TestUnknownAttempt：网关结果未知时保持原 attempt，重复触发不新增调用或扣费请求。
- [ ] 在 `backend/` 执行以下命令，确认用例已被运行并因目标行为缺失而失败。

  ```sh
  go test -race ./tests/... -run '^(TestRunAdmission|TestUnknownAttempt)$'
  ```
- [ ] 事务内做版本、权限与预算检查并保存运行，Workflow ID 与幂等键绑定。调用前写入尝试标识。若运行已登记但启动中断，恢复同一运行。
- [ ] 重跑上述 Go 测试，确认列出的用例均通过。
- [ ] 并行、缺额和启动失败样本通过，编辑页面可继续手工操作。

### Task T-02 运行三步制作 Agent 并人工采纳

**依据与依赖**　R-01 / FR-01 / AC-01。NFR-01 / AC-11。前置为 T-01。011 T-01、T-02。012 T-02、T-03。对照本专题需求与 PRD。**状态**　未开始。

**Files**

新增

```text
backend/internal/agent/application/agent/script.go
backend/internal/agent/application/agent/storyboard.go
backend/internal/agent/application/agent/continuity.go
backend/internal/agent/adapter/provider/production.go
frontend/src/features/studio/production_candidates.tsx
```

修改

```text
backend/internal/agent/application/agent/production_workflow.go
```

**Test**

```text
backend/tests/agent/production_agent_candidates_test.go
```

**Interfaces**　固定候选结构包含场景对白、镜头提案、冲突或依据不足。记录 `input_version_id`、`config_version` 与 `Evidence[]`。调用 011 T-02 的 RecordCandidate，接受动作通过 ApplyCandidate 完成。

- [ ] TestAgentCandidateBoundary：有证据冲突与无证据分别可识别。包含发布、交易指令的候选不会执行越权工具。作者拒绝候选后正式快照保持原样。
- [ ] 在 `backend/` 执行以下命令，确认用例已被运行并因目标行为缺失而失败。

  ```sh
  go test -race ./tests/... -run '^(TestAgentCandidateBoundary)$'
  ```
- [ ] 实现三种结构化输出、明确的工具白名单和授权 RAG 输入。在剧本、分镜及审片分别接入生成草稿、转为分镜及检查一致性入口。候选列表支持编辑、拒绝与显式采纳。获准媒体生成只回写候选引用。采纳时再次校验输入版本和使用权。
- [ ] 重跑上述 Go 测试，确认列出的用例均通过。前端变更在 `frontend/` 运行 `pnpm lint`、`pnpm types:check`、`pnpm test`，并完成相应页面场景。
- [ ] 模拟三步后人工分别采用和拒绝。保留引用版本、候选和采纳记录。

### Task T-03 提供任务状态与步骤恢复

**依据与依赖**　R-02 / FR-02 / AC-02。NFR-03 / AC-13。前置为 T-01、T-02。对照本专题需求与 PRD。**状态**　未开始。

**Files**

新增

```text
backend/internal/agent/adapter/http/assistant/run_events.go
frontend/src/features/studio/production_run_drawer.tsx
```

在 `backend/schema.sql` 更新对应业务表。

修改

```text
backend/internal/agent/application/agent/production_workflow.go
```

**Test**

```text
backend/tests/agent/production_agent_recovery_test.go
```

**Interfaces**　GetProductionRun(run_id) → 输入版、步骤、引用、费用状态。CancelProductionRun(run_id) → 取消状态。SSE 有 run_id、event_id 和 step_status，重连可取持久状态。

- [ ] TestStepRecovery：分镜失败时保留编剧产物，明确重试只重跑失败步骤，输入改版后迟到结果为待复核。TestRunEventsAccess：刷新、断线重连或无权订阅不丢权威状态且不泄密。
- [ ] 在 `backend/` 执行以下命令，确认用例已被运行并因目标行为缺失而失败。

  ```sh
  go test -race ./tests/... -run '^(TestStepRecovery|TestRunEventsAccess)$'
  ```
- [ ] 持久化状态与审计后再推送。取消与已完成回调的先后结果可解释，失败原因区分可重试与待核对。详情展示脱敏后的输入及引用。
- [ ] 重跑上述 Go 测试，确认列出的用例均通过。前端变更在 `frontend/` 运行 `pnpm lint`、`pnpm types:check`、`pnpm test`，并完成相应页面场景。
- [ ] 以一例失败运行追踪到步骤、版本、证据及计费状态，核对前端和日志的暗字段。

### Task T-04 核对用量与取消后的费用

**依据与依赖**　R-03 / FR-03 / AC-03。NFR-02 / AC-12。NFR-03 / AC-13。前置为 T-01、T-03。对照本专题需求与 PRD。**状态**　未开始。

**Files**

新增

```text
backend/internal/agent/application/usage/production_usage.go
frontend/src/app/manage/ai-usage/production/page.tsx
```

在 `backend/schema.sql` 更新对应业务表。

修改

```text
backend/internal/agent/adapter/provider/production.go
```

**Test**

```text
backend/tests/agent/production_agent_usage_test.go
```

**Interfaces**　ReconcileProductionUsage(attempt_id, provider_request_id, usage) → settled 或 pending_review。凭已确认结果结算或释放，未知保留核对记录。

- [ ] TestUsageReconciliation：已知、未知、取消和晚到回报均可追踪，重复回报不重复结算，待核对前不重发外部调用，页面及日志不返回密钥。
- [ ] 在 `backend/` 执行以下命令，确认用例已被运行并因目标行为缺失而失败。

  ```sh
  go test -race ./tests/... -run '^(TestUsageReconciliation)$'
  ```
- [ ] 按 attempt_id 对齐成本、预占和实扣。运营视图关联步骤和供应商记录。真实调用只在批准的测试额度内追加。
- [ ] 重跑上述 Go 测试，确认列出的用例均通过。前端变更在 `frontend/` 运行 `pnpm lint`、`pnpm types:check`、`pnpm test`，并完成相应页面场景。
- [ ] 在网关模拟下对账通过，审核后另录授权环境的供应商调用凭证。

## 验收覆盖与证据

| 验收依据 | 任务 | 核对方法 | 结果 |
| --- | --- | --- | --- |
| AC-02、AC-03、AC-12 | T-01 | 并行、缺额和启动失败样本通过，编辑页面可继续手工操作。 | 未执行 |
| AC-01、AC-11 | T-02 | 模拟三步后人工分别采用和拒绝。保留引用版本、候选和采纳记录。 | 未执行 |
| AC-02、AC-13 | T-03 | 以一例失败运行追踪到步骤、版本、证据及计费状态，核对前端和日志的暗字段。 | 未执行 |
| AC-03、AC-12、AC-13 | T-04 | 在网关模拟下对账通过，审核后另录授权环境的供应商调用凭证。 | 未执行 |

运行样本显示已知冲突和依据不足，失败可恢复，未知用量可核对。保留手工制作路径。 实际证据随任务记录版本、环境、样本、操作、结果和链接，区分模拟、隔离环境与授权试点。

## 交付与交接

计划由 `workspace-plan` 与 `superpowers:writing-plans` 编写，已保存供评审。实施、业务验收与发布均待执行。共享依赖和联合验收顺序见[专题索引](../00-文档导航.md#学屿原创-ip-制作与共创-saas)。
