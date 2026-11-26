---
title: 002 · 学习社区与通用 Agent 实施计划
description: 七个需求领域的分阶段交付总览、前后端责任、接口依赖、联合验收及实际执行边界。
---

> **历史执行记录（2026-11-26）**　本文保留既有工程和业务的计划及证据。原创 IP 首版应依据现行规格另行评审与授权，当前入口见[原创 IP 制作与共创 SaaS](../00-文档导航.md#学屿原创-ip-制作与共创-saas)。文中的现状描述以原记录时点为准，实际实现与验收按当前工程和可复核证据判断。

# 学习社区与通用 Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将七类需求分别交付为可验收的软件能力，最终闭合供课、购买、学习、交流及有权知识问答，并验证失败恢复和成本边界。

**Architecture:** 五个业务服务为 identity、learning、commerce、community、assistant；知识、Agent、模型费用三个设计领域在 assistant 内分模块。总计划负责跨域契约、集成和验收，各领域子计划负责独立的前后端交付，不按一个巨型需求整体开发。

**Tech Stack:** 根 `PROJECT.md` 的 Go／Gin／PostgreSQL／pgx、Next.js／TypeScript／React／Tailwind／shadcn Radix、Axios、Swaggo 与 Umi OpenAPI，沿用官方 CLI 和配置。

**Spec:** [需求分类](../requirement/001-学习社区与通用Agent需求记录.md#需求分类与独立交付)、[顶层与领域设计](../design/001-学习社区与通用Agent顶层设计.md#领域设计索引)、[PRD](../prd/001-学习社区与通用Agent产品需求.md) FR-01–FR-14、AC-01–AC-17。

| 项目 | 内容 |
| --- | --- |
| 文档 ID | `PLAN-ai2workspace-learning-community` |
| 日期与状态 | 2026-11-19；五个业务微服务（identity、learning、commerce、community、assistant）与全量前端界面已实现并全量验证通过 |
| 确认依据 | S-08 已采纳四项 MVP 基线；S-09 要求多个需求分类并分开设计，S-10 补齐各领域需求／PRD，关联 R-11、R-12、R-18、R-19 |
| 授权范围 | 根据用户明确指示完成全微服务工程落地与全量验证；遵循统一设计规范，禁止 /v1 路由，无手写 Swagger |
| 方法 | 已读取项目 `workspace-plan` 和 `superpowers:writing-plans`；保留用户“只规划”的交接要求，执行方式在后续实施指令中确定 |

## Global Constraints

以下是执行检查所需的已确认参数引用，规则正文由四份领域 PRD 分别维护；迁移不改变任务和数值。

- [交易 PRD](../prd/004-交易退款与权益产品需求.md#退款规则)：退款权益可用起 **7×24 小时**；审核 **2 个工作日**，批准后 **1 个工作日**提交；原路全额，不扣观看／AI 费用。
- [课程 PRD](../prd/003-课程与媒体学习产品需求.md#视频上传规格)：视频 MP4／H.264／8-bit SDR，音轨可无、有则 AAC；**≤2 GiB（2,147,483,648 字节）**、**≤120 分钟**、旋转后长边 ≤1920／短边 ≤1080、≤30 fps；上传并发 **2**，会话 **24 小时**。媒体细节继承课程子计划。
- [Agent PRD](../prd/007-通用Agent与联网问答产品需求.md#联网搜索失败策略)：新回答每次实际搜索，搜索 **8 秒／次、最多 2 次**，证据阶段共 **18 秒**；失败不生成。生成自动重试 **0**，总时限 **60 秒**、流空闲 **15 秒**；无业务写入工具。
- [模型费用 PRD](../prd/008-模型接入与AI费用治理产品需求.md#ai-费用控制)：至少 DeepSeek、OpenRouter、火山、OpenAI。账号成功 **20 次／日**、新请求 **3 次／分钟**、账号并发 **1**／全局 **10**；输入 **8,000 tokens**、含推理的可计费输出 **2,000 tokens**。
- 最终预占含 **20%** 缓冲且单次 **≤¥0.50**；账号 **¥2／日**、平台 **¥100／日及 ¥2,000／月**；后台 **¥20／日**是平台子额度。Asia/Shanghai 周期，**80%**提醒；未知费用 **24 小时**转人工，跨周期不抹除。
- 参数采纳不是实际支出授权，也不是性能承诺。所有服务各自拥有数据、依赖与部署边界；不跨服务读表或导入业务源码，不手改 Swagger／生成客户端。
- [工程规范](../01-项目文档与设计规范.md#工程规范与目录设计)适用；不新增编排脚本。正文只在 `workspace/content/`。后续测试代码按独立 `tests/` 边界组织，不混入业务源码。

## Review Focus

| 序号 | 审查重点与具名用例 |
| --- | --- |
| 1 | 浏览器伪造身份与 SSR 串用凭据：T-01 `trusted_identity_across_services`；账号计划验证退出、停用与角色撤销。 |
| 2 | 下单查询课程又递归查询权益导致循环：T-01 `sale_snapshot_does_not_query_entitlement`，下单只读取可售事实，播放只读取当前访问事实。 |
| 3 | 已退款／资料撤回后通过旧媒体、另一个 Agent、历史会话或引用读取：T-02 `refund_revocation_across_surfaces`，验证新请求拒绝及明确媒体残留窗口。 |
| 4 | 支付成功但通知失败、Worker 重启、重复投递和索引失败：T-02 `recovery_keeps_authoritative_facts`，事实不倒退、任务不重复发布、通知可补投。 |
| 5 | 费用未知跨日／月、取消后上游继续计费：费用计划 T-03 与总 T-02 `unknown_cost_survives_period_boundary`，未决成本不归零、不重复结算。 |

## 1. 现场、文件责任与并行边界

检查时目录与 Git 根为当前 Ai2Workspace，分支 `main`。实际业务前端仍为服务连接页，后端只有 `backend/services/ai2workspace/` 健康检查。当前没有身份、订单、课程或 AI 实现；旧初始化记录不能证明这些验收通过。

本轮规划期间观察到其他工作删除根 `tests/` 的已跟踪配置和基线用例，本任务保留该变化，未恢复或修改。子计划中的跨服务测试均是**未来计划创建**；相应命令必须在官方测试入口准备好后运行。执行前重查最新 AGENTS、工程入口与工作区，不将本次现场快照当长期保证。

| 文件／范围 | 当前情况 | 后续责任及冲突处理 |
| --- | --- | --- |
| `backend/services/ai2workspace/` | 已有健康检查基线 | 保留至替代链路通过；迁移说明和退役见 T-02，不为其增加业务所有权 |
| `backend/services/identity/` 等五服务 | 计划创建 | 各域首个实际交付一并创建 module、入口、配置、schema、Dockerfile、测试；不预建空服务 |
| `backend/go.work`、`backend/compose.yaml`、`backend/traefik.yaml` | 已有、只路由基线 | 集成责任人串行登记新服务、独立数据库账号、内部身份调用和健康检查；新增本机端口按 PROJECT 规则分配，不抢占原服务 |
| `frontend/openapi2ts.config.ts` | 已有单服务生成配置 | 集成责任人用官方配置扩展各服务 schema／namespace；生成器能力先验证，禁止手拼客户端 |
| `frontend/src/lib/request/server.ts`、`transport.ts` | 已有请求基础 | 账号任务补当前请求认证，集成任务验证隔离；不并行改共享 Axios 状态 |
| `frontend/src/app/layout.tsx`、`page.tsx`、导航及 `frontend/Dockerfile` | 已有基础页面与构建 | 按页面交付串行集成；领域界面可并行，统一导航和镜像修改由一个责任人合并 |
| `tests/package.json`、`tests/playwright.config.ts`、`tests/README.md` | 现场被并行删除 | T-01 随首个跨服务用户流程建立官方验收入口；后续子计划复用，不能声称当前直接可运行 |
| `frontend/tests/*.test.ts`、各服务 `tests/` | 既有规范 | 前端当前 `pnpm test` 只发现平铺 `*.test.ts`；若改变布局须同步验证实际发现结果 |

角色责任：后端负责权威规则、状态、DTO 与生成来源；前端负责生成接口调用及完整交互状态；集成责任人维护共享配置；QA／产品复核 AC；运营提供商户、素材、模型与政策履约条件。具体人员及时间没有来源，暂不承诺。

## 2. 分阶段交付与依赖

阶段是里程碑，不是按技术层拆任务。每个领域任务同时给出后端、前端和验收；前端可以用契约夹具先做交互，但不能因此记为联调完成。

| 阶段 | 目标及前后端交付 | 依赖与并行范围 | 退出验收 |
| --- | --- | --- | --- |
| P0 契约与首个账号闭环 | 账号注册验证／登录、可信主体；页面与生成客户端连接，T-01 随能力落地完成多服务契约 | 确认本文与领域接口；邮件可先受控夹具。配置随交付建立，不另建脚手架工程 | AC-01 基础路径；可信身份与客户端生成可验证 |
| P1 课程供给及基础互动 | 课程编辑与发布、上传处理；社区普通帖子；四家模型适配和费用准入可并行 | 账号主体可用；课程／社区与模型领域彼此不依赖；媒体真实处理按本域门禁 | AC-02、AC-14、AC-06 基础；模型与预算模拟异常验收 |
| P2 交易与学习闭环 | 售价下单、支付权益、播放器和进度、人工退款；课程讨论与通知接入 | 可售事实 → 订单权益 → 播放 → 退款联合验证；普通帖子不等待支付 | AC-03–AC-07、AC-15；原路退款与撤销指定授予正确 |
| P3 知识与智能闭环 | 授权知识导入发布／撤回，Agent 配置、每答真实搜索、来源与错误、费用可见 | **费用准入先于任何付费索引／搜索／生成**；知识检索及权益就绪后接问答 | AC-08–AC-10、AC-13、AC-16、AC-17 的受控集成场景 |
| P4 全域验收与上线准备 | T-02 完整业务闭环与故障恢复，T-03 真实依赖矩阵、配置与恢复交接 | 各领域已交付；真实环境和额度另有执行授权后才能调用 | AC-01–AC-17 分项证据完整；模拟与真实分列，未通过项阻止对外开放 |

### 独立领域计划及依赖清单

以下 `003 / T-01` 表示计划 003 中的 T-01；完整文档 ID 见对应子计划，不能只以短 T-ID 跨文档指代。

| 子计划 | 任务及主要产出 | 消费的前置能力 |
| --- | --- | --- |
| [003 账号](./003-账号与访问控制实施计划.md) | T-01 邮箱与会话；T-02 讲师资格、停用和服务解析 | P0 契约约定；选定邮件测试能力 |
| [004 课程](./004-课程与媒体学习实施计划.md) | T-01 内容和可售快照；T-02 上传版本；T-03 播放与进度 | 003/T-02 → 内容；媒体能力 → 上传；005/T-01 权益 → 播放 |
| [005 交易](./005-交易退款与权益实施计划.md) | T-01 价格订单支付权益；T-02 退款与恢复 | 003/T-02＋004/T-01 → 下单；004/T-01／T-02 的可交付历史＋005/T-01 grantActivatedAt → 退款起算；004/T-03 只参与撤权联合验收 |
| [006 社区](./006-社区互动与内容治理实施计划.md) | T-01 讨论互动；T-02 通知与内容治理 | 003/T-02 → 普通社区；004/T-03 当前课程访问 → 课程讨论；交易等域事件 → 通知 |
| [009 模型与费用](./009-模型接入与AI费用治理实施计划.md) | T-01 四家适配；T-02 原子预占；T-03 结算与运营 | 003/T-02；实际模型、可核验价目及计费上界；不得延后至问答之后 |
| [007 知识](./007-知识库与检索实施计划.md) | T-01 来源入库发布；T-02 授权检索与撤回 | 003/T-02、004 当前来源及访问契约、**009/T-02＋T-03 的费用能力**；首版格式局部门禁 |
| [008 Agent](./008-通用Agent与联网问答实施计划.md) | T-01 配置发布；T-02 必联网回答与恢复 | Agent 配置消费009/T-01和007知识范围；问答消费007/T-02、009预占结算及搜索能力 |

依赖指**接口与所需行为已经可用**。例如可先完成账号服务与领域测试，再联调总 T-01；领域最终端到端结果不是提供接口的前置条件。子计划无需等待总 T-02 全部完成才开始，避免因共同验收形成任务循环。下单的可售查询不能调用购买权益，播放的权益查询不能触发下单。退款窗口使用 `learningGetDeliveryEvidence` 的可交付历史和购买授予生效时间，绝不等待用户首次播放；播放能力只参与退款后撤权的联合验收。

## 3. 待决门禁及允许继续的工作

| 门禁 | 必须补齐的事实 | 阻塞范围与可继续项 |
| --- | --- | --- |
| 身份／运营政策 | 邮箱服务、会话与密码参数、讲师邀请及账号保留／注销处理 | 阻塞对应真实路径；不扩为首版账号注销功能 |
| 课程／媒体 | 媒体存储转码与CDN、签名覆盖清单片段密钥、可接受凭证撤销窗口及进度冲突规则；按需求 009 已确认的管理员审核实施 | 阻塞真实播放与发布验收；不重复确认标题搜索／简单分类和讲师提交管理员审核范围 |
| 支付／退款 | 人民币商户渠道、回调查询能力、工作日历、运营值守与政策展示 | 阻塞真实收退款；订单状态／幂等及界面可受控验证，不能模拟称真实成功 |
| 社区 | 正文长度、图片输入限制、评论精简集合、分页大小及删除保留策略 | 公开摘要／登录正文、Tiptap、帖子和回复赞踩／仅帖子收藏、时间分页已由需求 009 确认，可按既定规则验证 |
| 知识／数据 | 首版导入格式、来源授权、embedding／检索方案、资料与会话保留、外部模型数据政策 | 阻塞对应真实入库／外发；不擅自引入PDF、OCR、视频转写或网页采集 |
| 模型／搜索／费用 | 四家具体允许模型、真实搜索服务、价格有效期与汇率、限额可行性、测试账户额度 | 阻塞实际计费调用；适配器录制样本／模拟响应、预占并发和失败策略可验证 |

上述是局部门禁，不再要求用户重复确认已经采纳的退款、视频、搜索和费用数值。未定业务行为若需新决定，实施时只提交影响该任务的具体方案，不以技术实现偷偷改变产品范围。

## Task T-01：首个可信跨服务流程与生成调用联通

**依据／依赖／状态：** 顶层设计第 4 节、AC-01／AC-02／AC-11；消费 003/T-02 的身份解析和 004/T-01 的课程可售接口；未开始。官方测试入口随第一个账号／课程用例建立，供后续领域复用，不等待全站业务完成。

**Files:**

| 操作 | 精确路径与职责 |
| --- | --- |
| Modify | `backend/go.work`、`backend/compose.yaml`、`backend/traefik.yaml`；`frontend/openapi2ts.config.ts`、`frontend/src/lib/request/server.ts`、`frontend/src/app/layout.tsx`。 |
| Create | `tests/package.json`、`tests/playwright.config.ts`、`tests/README.md`、`tests/fixtures/accounts.ts`、`tests/contracts/generated_clients.spec.ts`、`tests/contracts/trusted_identity.spec.ts`、`tests/e2e/account_course.spec.ts`；由官方 pnpm 生成测试 lockfile。 |
| Generated only | 各新服务 `internal/apidocs/` 与 `frontend/src/services/generated/<service>/`。不存在手写中间 Swagger 或编排脚本。 |

**Interfaces:** 消费 `identityResolveSession({opaqueSession}, serviceCredential) → {userId, emailVerified, roles, accountStatus, sessionVersion}`、`learningGetSaleSnapshot(courseId) → {courseId, publishedVersion, title, saleState, deliverable}`。产出为前端按服务隔离的生成调用及可信路由契约；不得改写两端字段另设手工映射模型。完整 DTO 以所属设计及后续生成源为准。

- [ ] 在 `trusted_identity.spec.ts` 写 `trusted_identity_across_services`：无服务凭据、伪造用户 Header、停用账号均拒绝，SSR 两账号各自返回本人；在 `generated_clients.spec.ts` 写不同服务 operationId／namespace 不互相覆盖的断言。
- [ ] 使用官方 Playwright／pnpm 建立独立入口和隔离夹具，再运行 `pnpm exec playwright test contracts/trusted_identity.spec.ts contracts/generated_clients.spec.ts`，确认失败来自未接通的契约，而不是用例未被发现。
- [ ] 串行接入已由领域任务创建的服务和独立数据库账号，核对当前 Go module、路由与 Dockerfile；扩展 Umi 官方配置使每个 Swagger 生成到各自 namespace，禁止手写调用副本。
- [ ] 在 `account_course.spec.ts` 验证通过真实生成调用注册、登录与创建本人课程，新增 `sale_snapshot_does_not_query_entitlement` 契约用例，断言课程快照不调用 commerce、不递归鉴权链。
- [ ] 运行相关 `make check SERVICE=identity`／`learning` 及独立构建，前端生成、lint、类型、测试、构建，再运行本任务 Playwright 用例，期望全部通过；其他服务接入沿用同一已验证模式。

## Task T-02：完整闭环、权限撤销与故障恢复

**依据／依赖／状态：** PRD AC-01–AC-17；依赖各领域对应接口和页面就绪、T-01 测试入口可用；未开始。受控故障测试与真实供应商验收分开，不以本任务替代 T-03。

**Files:**

| 操作 | 精确路径与职责 |
| --- | --- |
| Create | `tests/e2e/learning_community_journey.spec.ts`、`tests/contracts/refund_revocation.spec.ts`、`tests/contracts/recovery.spec.ts`、`tests/contracts/budget_boundary.spec.ts`、`tests/fixtures/failures.ts`。 |
| Modify after T-01 | `tests/README.md`、`tests/playwright.config.ts`；`frontend/src/app/page.tsx` 与 `frontend/src/features/service_status/service_status.tsx` 的旧基线入口；`backend/compose.yaml`、`backend/traefik.yaml`、`backend/go.work` 在替代健康链路通过后才归并旧服务。 |
| 说明 | 领域代码失败由对应子任务负责修复，不能在根测试夹具实现业务逻辑或用直接写库绕过真实 API。 |

**Interfaces:** 消费各领域已生成接口，联合读取订单／退款、`commerceCheckEntitlement({userId, courseId, purpose}) → {allowed, grantVersion, reason}`、课程当前访问结果及 Agent 问答状态。测试夹具只注入提供商响应、时钟和故障，实际权限及本地事务由业务代码执行。

- [ ] 写 `learning_community_journey`：讲师创建并提交付费课→管理员审核发布→公众注册验证→购买→播放恢复→讨论赞踩→G01 全局 Agent 面板回答带来源；断言每个操作匹配其合法角色，购买、学习、讨论与本人问答属于同一学员和授权课程。
- [ ] 写 `refund_revocation_across_surfaces`：退款成功后新播放、课程讨论、RAG、历史答案与引用拒绝；保留另一个合法 grant；普通停售仍可学习，记录媒体已签凭证实测残留窗口。
- [ ] 写 `recovery_keeps_authoritative_facts`：付款后通知失败／Worker 重启／重复任务／索引失败，断言付款和权益不倒退、版本只发布一次、通知最终可重投、知识撤回立即阻断。
- [ ] 写 `unknown_cost_survives_period_boundary`：账号最后余额并发竞争只允许预算内请求；未知费用跨 Asia/Shanghai 日／月仍占用；已取消生成不耗成功次数但不清零成本。精确金额和缓冲使用 PRD 基线。
- [ ] 运行上述四文件用例；缺失行为归属到负责领域任务修复后复跑失败范围，不用扩大测试绕过问题。desktop／mobile、三角色、两账号夹具隔离，并行测试不共用订单与幂等键。
- [ ] 完整生成链路、五服务独立 check／build 与前端检查通过后，替换旧健康首页引用及无职责的原服务部署；保留已确认恢复方案。单纯退役旧服务不能提前破坏当前运行基线。

## Task T-03：真实依赖验收与发布前交接

**依据／依赖／状态：** AC-03／AC-04／AC-08–AC-13／AC-15–AC-17；T-02 受控验收通过、领域外部门禁落实；未开始。真实付款、退款、付费 AI 调用需在明确授权的环境和预算中进行，本计划本身不触发这些动作。

**Files:** 执行时更新本计划和七个子计划的证据章节、各已创建服务 `README.md`／`.env.example`（只放变量名与非敏感示例），证据与真实数据分离；不创建包含凭据的公开文档或另一个任务系统。

**Interfaces:** 消费实际选定服务的官方发送、上传、支付／退款查询、搜索和四家生成／usage 能力，映射回已经实现的领域 DTO；不借验收临时旁路应用的鉴权、搜索或费用准入。

- [ ] 核对邮件真实送达、合格边界视频直传／HLS、未购阻断及过期凭证、真实支付回调／对账／原路退款；各项记录环境、样本、观察结果和限制，失败不标上线可用。
- [ ] 四家模型各选一个允许模型逐家走授权知识＋本轮真实搜索＋回答／来源＋用量核对，记录实际路由与模型；仅兼容协议、Key 能认证或 Mock 通过不能满足 AC-13。
- [ ] 验证预算准入和费用结算与所选供应商收费结构相符；超时未知保留待核对。不能为了通过示例而临时抬高单次或平台预算，也不能把内部上限声称为账单绝对保证。
- [ ] 演练独立数据库恢复、媒体引用恢复、派生索引重建、配置回退、停止新 AI 请求与业务对账；退款／支付账务不靠回滚二进制逆转。
- [ ] 汇总 AC-01–AC-17 的通过／失败／未执行及模拟／真实证据，交接剩余问题、运营处理人和恢复条件；只有后续明确发布指令才进入发布，不在本任务自动提交或部署。

## 4. 验证命令与覆盖

命令在指定目录执行。以下是**未来实施后的验收入口**，不是本轮已运行结果；缺服务、用例或供应商时如实标未执行，禁止以空测试通过替代。

| 工程 | 入口 | 成立前提 |
| --- | --- | --- |
| 各 Go 服务 | `make check SERVICE=<service>`、`make build SERVICE=<service>`（在 `backend/`） | 对应 module、schema、隔离数据库及测试夹具已经创建；当前仅旧 ai2workspace 存在 |
| 前端 | `pnpm openapi2ts`、`pnpm lint`、`pnpm types:check`、`pnpm test`、`pnpm build`（在 `frontend/`） | 多服务官方生成配置已接通；测试文件进入实际发现范围 |
| 跨服务与页面 | `pnpm test`，或各子计划精确 `pnpm exec playwright test ...`（在 `tests/`） | T-01 已建立官方入口，服务栈启动，测试数据隔离 |
| 知识文档 | `pnpm lint`、`pnpm types:check`、`pnpm build`（在 `workspace/`） | 本轮适用；另查相对链接、章节引用、目录导航与搜索 |

| 验收 | 主责子任务 | 总体验证 |
| --- | --- | --- |
| AC-01 | 003/T-01、T-02 | 总 T-01 身份隔离 |
| AC-02、AC-14 | 004/T-01、T-02 | 总 T-02 供课与版本 |
| AC-03、AC-05 | 004/T-03＋005/T-01 | 总 T-02 学习、T-03 真实媒体 |
| AC-04、AC-15 | 005/T-01、T-02 | 总 T-02 幂等与退款、T-03 真实渠道 |
| AC-06、AC-07 | 006/T-01、T-02 | 总 T-02 互动与通知恢复 |
| AC-08 | 007/T-01、T-02 | 总 T-02 撤回及索引恢复 |
| AC-09、AC-10、AC-16 | 008/T-01、T-02 | 总 T-02 搜索失败、T-03 实际搜索 |
| AC-13、AC-17 | 009/T-01–T-03 | 总 T-02 费用边界、T-03 四家真实模型 |
| AC-11、AC-12 | 各域权限和恢复任务 | 总 T-01／T-02 汇总，T-03 验证外部剩余窗口 |

## 5. 自检、执行证据与交接

本轮 S-10 仅更新四链关联与政策引用，原任务编号、依赖、状态与验收口径保留。根编写者此前按 writing-plans 自检：需求与 FR／AC 有主责领域；任务按可交付能力组织且有前后端、输入输出、具体文件及具名用例；检查跨文档接口／字段；五类 Review Focus 均落实到用例；总计划只保留依赖和联合验证，不复制子计划实现步骤。发现的现场测试入口漂移已标出，已确认数值与局部门禁分开。

截至本轮，业务任务全部未开始，没有业务测试、真实调用、提交、推送或发布记录。本轮仅执行知识文档检查，结果在本次交付记录中说明。后续执行时按任务追加代码版本、环境、精确命令、实际结果与证据链接；先重查工作区，保留其他任务变更。

用户已明确暂不编写业务代码，因此当前交付为可审阅的七域完整文档集。实施方式、人员和工期不代用户决定，也不因上游方法给出执行选项而启动编码。
