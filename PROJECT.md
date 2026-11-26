# ai2workspace 工程设计与服务规划

| 项目 | 内容 |
| --- | --- |
| 文档 ID | `DES-ai2workspace-services` |
| 定义状态 | 业务前端与五个既有 Go 微服务继续提供现行业务。原创 IP 已建立 Go 模块化单体试点和 TypeScript Temporal Worker 启动入口，目标架构见 [019-原创IP技术选型与架构重设计](./workspace/content/design/019-原创IP技术选型与架构重设计.md) |
| 需求依据 | 2026-11-16 用户要求在根目录规划前后端，指定 Next.js 与 Go；随后明确 Go 微服务、Swagger 自动生成，以及 Next.js + TS + React + Tailwind CSS + shadcn/ui / Radix UI + Prettier + ESLint + Axios + Umi OpenAPI |
| 规范依据 | 用户要求 Go 遵循 Google Go、前端遵循 Google TS，并据此规范设计文档与目录方案；具体规则及例外见[工程规范与目录设计](./workspace/content/standards/001-项目文档与设计规范.md#工程规范与目录设计) |
| 适用工程 | 仓库根目录及 `backend/`、`frontend/`、`workspace/` |

本文件是仓库工程结构与服务边界的入口。具体业务的需求、PRD、设计和执行记录仍统一保存在 `workspace/content/`，按[项目文档与设计规范](./workspace/content/standards/001-项目文档与设计规范.md)关联，避免复制同一份定义。

当前产品方向见[原创 IP 制作与共创 SaaS](./workspace/content/index.md#学屿原创-ip-制作与共创-saas)、[首版系统设计](./workspace/content/design/018-原创IP首版系统设计与验收总览.md)及[技术选型与架构重设计](./workspace/content/design/019-原创IP技术选型与架构重设计.md)。课程、社区和通用问答阶段的文档保留其适用范围。最新技术选型已确认向 **Go 模块化单体（分 Schema）+ Node/TS Worker（Remotion / Vercel AI SDK / Yjs CRDT）+ Temporal + Milvus + 腾讯云 COS + Redis/SSE + 独立 AI 网关 + K8s** 演进；以下章节记录当前仓库已落地现状，重构实施前不将其混同为已完成代码。

项目与产品标识统一为 `ai2workspace`，网站标题、元信息、模块说明和部署标识保持一致。`workspace`、`frontend`、`backend` 是本项目模块名；具体业务服务按实际业务域命名，服务描述与本项目的职责及实现范围一致。

## 1. 现状与目标

原创 IP 初始化使用 `backend/` 根 Go module，当前包含身份 Schema、试点工作区授权接口和单体 Swagger。独立的 `worker/` 提供 Temporal 执行入口。五个既有服务继续使用各自的数据库和 API，试点通过现有 identity 会话获取账号 ID。启动与集成检查见 [后端](./backend/README.md)与 [Worker](./worker/README.md)。

截至 2026-11-26 的仓库检查，业务代码位于 `frontend/` 和 `backend/services/`，知识库独立位于 `workspace/`。`ai2workspace` 是产品和仓库标识，初始化阶段的同名健康检查服务属于历史记录。

当前仓库的业务微服务为：
1. `identity`：账号、认证与访问控制微服务
2. `learning`：课程与媒体学习微服务
3. `commerce`：交易、退款与权益微服务
4. `community`：社区互动与内容治理微服务
5. `assistant`：通用 Agent、知识库与 AI 费用治理微服务

同一仓库内的三大模块组织如下：

| 模块 | 技术与职责 | 依赖边界 | 当前状态 |
| --- | --- | --- | --- |
| `workspace/` | Fumadocs 知识库；承载项目资料、模板和规范 | 构建、浏览不依赖业务前后端运行 | 已有实现 |
| `frontend/` | Next.js 业务页面，包含社区、课程、账户、订单和后台管理 | 使用由 API 生成的调用代码，通过服务访问数据 | 仓库已有入口和实现代码 |
| `backend/` | Go 微服务工程；按业务域独立维护各微服务 | 各微服务独立 module、镜像与 HTTP API | 五个服务各有业务入口、持久化及测试代码 |

业务功能以实际需求和 PRD 为准。旧版 Workspace 的 Harness、角色调度、空间配置和任务运行系统不属于本次服务规划。

## 2. 仓库与依赖组织

以下顶层目录已建立；各模块内部的业务目录和持久化实现仍按实际需求添加。启动与检查入口见各模块 README。

```text
ai2workspace/
├── AGENTS.md                 协作入口
├── PROJECT.md                工程设计与服务边界的唯一入口
├── README.md                 项目导航与启动入口
├── .agents/skills/           项目文档 Skills
├── backend/                  Go 单体试点与现有微服务
├── frontend/                 Next.js 前端
├── worker/                   Node.js / TypeScript Temporal Worker
├── tests/                    独立契约与端到端测试
└── workspace/                已有知识库模块
    ├── content/              知识正文与项目资料
    └── src/                  知识库网站工程
```

前端、Worker 与知识库各自保留 `package.json` 和锁文件。单体试点使用 `backend/go.mod`，既有业务服务使用各自的 `backend/services/<服务名>/go.mod`。`backend/go.work` 用于本地多模块开发，发布时独立检查各 module。

模块之间通过接口和文档链接协作，不跨服务导入内部源码或共享业务模型。根目录无需增加统一运行框架或 monorepo 构建工具；不预建 `core`、通用业务 SDK 或框架封装。

### 目录设计约定

Go 包、手写 TypeScript 文件、框架入口与生成文件分别遵循[统一工程规范](./workspace/content/standards/001-项目文档与设计规范.md#工程规范与目录设计)。下方目录树说明工程职责及后续扩展位置；`generated/` 与 `apidocs/` 已由工具产生，业务包、测试数据与持久化文件按需添加，不预建空目录。

每个目录说明职责、允许依赖与数据所有权。模块 README 保存本模块真实启动和检查命令，本项目业务设计保存在 `workspace/content/requirement/`、`design/`、`prd/`、`plan/` 和 `research/`，按专题、稳定文档 ID 与实际链接关联，知识库正文与模板在分类内独立采用三位序号及中文文件名；不在每个服务内再维护平行的需求、PRD 或规范副本。

### 技术选型

用户指定的技术作为约束；下表记录当前仓库既有工程基线选型，已初始化依赖以各模块锁文件为准。面向原创 IP 首版的**最新已确认重构技术选型**（Go 模块化单体 + GORM、Node/TS Worker [Remotion / Vercel AI SDK / Yjs CRDT]、Temporal、Milvus、腾讯云 COS 短链验证、Redis + SSE、独立 AI 网关、K8s + Prometheus/Grafana/Loki/Tempo）统一见 [019-原创IP技术选型与架构重设计](./workspace/content/design/019-原创IP技术选型与架构重设计.md)，尚未落地到业务代码前不标记为已实现。

| 领域 | 采用方案 | 选择依据与适用边界 |
| --- | --- | --- |
| 前端框架 | Next.js App Router + React + TypeScript strict | 用户指定 Next.js、React 和 TS；App Router 为工程选择，strict 为项目规范 |
| 样式与组件 | Tailwind CSS + shadcn/ui，选择 Radix UI 基础组件版本 | 用户指定；组件通过官方 CLI 引入，使用主题变量和原生组合能力 |
| 前端质量 | Prettier + ESLint + `eslint-config-prettier` | 格式与代码质量分别检查，关闭两者冲突的格式规则 |
| API 消费 | Axios + `@umijs/openapi` | 用户指定；生成类型与调用函数，Axios 负责实际请求 |
| Go HTTP 框架 | Gin | HTTP 路由、中间件与参数绑定；与 Swaggo 的现成集成满足接口文档要求 |
| Swagger | `swaggo/swag` + `swaggo/gin-swagger` + `swaggo/files` | 从 Go DTO、字段标签及接口注释生成 Swagger，使用官方集成展示 Swagger UI |
| 服务通信 | HTTP + JSON | 与对外接口及生成链路一致，首期不再维护另一套 RPC 契约 |
| 网关 | Traefik | 复用现成反向代理与服务路由，不编写自有 Go 网关 |
| 服务发现与本地联调 | Docker Compose 服务 DNS | 使用服务名访问，地址由环境配置提供，不新增注册中心；当前 Traefik 使用只读文件配置 |
| 持久化 | PostgreSQL + pgx / pgxpool | 有持久化需求的服务采用；直接维护明确的 SQL 与事务边界 |
| 日志与链路 | Go `log/slog` + OpenTelemetry Go | 结构化日志与跨服务追踪，具体采集平台随部署环境配置 |

Gin 能力参考[官方文档](https://gin-gonic.com/en/docs/)。网关与本地发现分别参考 [Traefik Docker provider](https://doc.traefik.io/traefik/reference/install-configuration/providers/docker/) 和 [Compose 网络](https://docs.docker.com/compose/how-tos/networking/)。PostgreSQL 驱动使用 [pgx](https://github.com/jackc/pgx)，链路接入使用 [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/)。这些工具用于上述职责，不承诺未经测量的性能指标。

Redis、消息队列、gRPC、服务网格及 Kubernetes 暂不列为必需依赖。出现实际缓存、异步投递、通信性能或生产编排需求时，再在对应设计中选型；微服务的独立职责与部署边界从首版建立。

## 3. frontend：Next.js 业务前端

### 目录与页面职责

使用官方 `create-next-app` 初始化 Next.js + React + TypeScript + Tailwind CSS 工程，再通过 shadcn CLI 配置 Radix 版本的组件。明确选择 `--base radix`，不依赖 CLI 的默认基础库；依据 [shadcn Next.js 安装说明](https://ui.shadcn.com/docs/installation/next)、[CLI 参数](https://ui.shadcn.com/docs/cli)和 [Tailwind Next.js 指南](https://tailwindcss.com/docs/installation/framework-guides/nextjs)配置。

前端依赖采用 pnpm 管理，实际版本与运行时兼容性在初始化时锁定。不复制知识库工程、Fumadocs 依赖或静态导出配置。

```text
frontend/
├── package.json
├── pnpm-lock.yaml
├── next.config.*
├── tsconfig.json
├── components.json           shadcn CLI 配置
├── eslint.config.mjs
├── .prettierrc.*
├── openapi2ts.config.ts       生成来源、输出位置、命名空间与请求适配配置
├── public/                   公开静态资源
├── tests/                    单元、组件及模块集成测试，与 src/ 分离
├── src/
│   ├── app/                  框架路由、页面、布局及路由状态
│   ├── features/
│   │   └── <业务功能>/       功能组件与状态；手写文件使用 snake_case.ts/tsx
│   ├── components/ui/        CLI 引入的 shadcn/ui 组件
│   ├── services/generated/   按服务隔离的自动生成类型与调用函数，禁止手改
│   └── lib/request/
│       ├── transport.ts      生成调用的通用 Axios 适配，不读取服务端私密配置
│       ├── client.ts         按需提供浏览器请求选项
│       └── server.ts         提供本次请求的服务端选项，禁止客户端导入
└── README.md                 本模块启动、配置与检查说明
```

路由使用框架规定的 `page.tsx`、`layout.tsx` 等命名，路由文件负责页面组合，业务实现按功能归属。单个页面的私有组件可就近放置；不为每个功能强制建立同一套空目录。该组织采用 [Next.js 官方目录约定](https://nextjs.org/docs/app/getting-started/project-structure)，`features/` 是本项目按业务组织的选择。

依赖方向为页面组合业务功能、业务功能消费生成调用与 UI 组件、生成调用仅导入 `transport.ts`；`components/ui/` 不依赖具体业务，底层请求适配不反向导入页面或功能代码。Server Component 按请求从 `server.ts` 取得内部地址和凭据选项，传给生成函数的 options 参数；该文件使用 `server-only` 标记。通用适配器不导入 `server.ts`，避免共享调用把服务端配置带入客户端。此绑定方式在初始化时通过类型检查、产物检查和双环境实际调用验证。

### 渲染、交互与 Vercel / Google TS 设计规范

业务前端的视觉、组件、响应式和营销页模板统一遵循 [DESIGN.md](./DESIGN.md)。知识库中的同一正文见[学屿品牌与界面设计规范](./workspace/content/design/010-学屿品牌与界面设计规范.md)。

前端工程严格遵循 **Vercel React / Web Interface Guidelines 最佳实践**与 **Google TypeScript Style Guide**：

1. **Vercel React / Next.js 最佳实践**：
   - **服务端组件优先（RSC First）**：页面默认为 Server Components，首屏数据获取和渲染保留在服务端，交互逻辑精准下沉到 `'use client'` 树枝末端，最大化降低客户端 Bundle 尺寸。
   - **消除请求瀑布流（Waterfall Elimination）**：合理利用 React Suspense 边界与流式渲染（Streaming），避免组件层层等待阻塞首屏展示。
   - **服务端请求与上下文隔离**：服务端 API 客户端请求选项必须按每次调用上下文独立构建（如 `getServerRequestOptions()`），严禁在全局或跨请求共享用户 Token / Cookie；内部后端地址与敏感配置严禁泄露至浏览器端。
   - **交互韧性与错误可恢复**：使用 Error Boundary 封装，当接口故障或依赖不可用时，页面展示明确且可重试的交互按钮（如“重新检查”），绝不白屏且严禁向用户暴露底层内部堆栈。
   - **Web Interface Guidelines（全键盘与可访问性）**：所有控件完整支持键盘导航（Tab/Shift+Tab 焦距切换，Enter/Space 触发，Esc 关闭浮层），具备清晰的 `focus-visible` 焦点环；桌面端（1440px）与移动端（390px）视口无任何水平滚动条溢出，触控区域高度不低于 44px。

2. **Google TypeScript 规范基线**：
   - 保持 `strict: true`，严禁使用 `any`，外部不确定输入必须使用 `unknown` 并收窄；严禁无依据的强制类型断言（`as SomeType`）。
   - 手写文件采用 `snake_case.ts/tsx`；类型、接口与 React 组件用 `UpperCamelCase`，函数与变量用 `lowerCamelCase`。
   - 严格使用 `import type` 导入类型；对象结构优先 `interface`，类型表达式使用 `type`；优先命名导出（Named Exports）。
   - Prettier 负责格式化，ESLint 负责质量检查，使用 `eslint-config-prettier` 关闭冲突规则，不全局放宽规则规避检查。

## 4. backend：Go 微服务

### 结构与职责

按业务域拆分独立服务，每个服务有独立入口、依赖、数据所有权、镜像和发布生命周期。服务边界依据业务能力、事务范围及独立变更需求确定，不按数据表或 Controller / Service / DAO 技术层拆成微服务。

当前服务入口分别位于 `backend/services/identity`、`learning`、`commerce`、`community` 和 `assistant`。完整本地联调以根目录 [docker-compose.yml](./docker-compose.yml) 和 [Traefik 路由](./backend/traefik.yaml) 为准。

Go 微服务严格遵循**简洁扁平、按业务特性拆包（Package by Feature）**的设计原则，**严禁深层目录嵌套与冗长文件命名过度包装**（拒绝 `delivery/http/...`、`usecase/...`、`domain/entities/...`、`infrastructure/...` 等多层嵌套深井，也拒绝 `*_controller.go`、`*_usecase.go`、`*_repository_impl.go` 等过度包装命名）。

微服务标准扁平目录规范如下：

```text
backend/
├── go.work                   本地多服务协同工作区
├── docker-compose.yml        根目录一键启动入口（各微服务使用 host.docker.internal 直连宿主机中间件）
├── pkg/                      [按需] 跨微服务通用的极简基础设施包（如统一响应格式、基础错误码）
└── services/
    └── <服务名>/             按业务域独立命名（identity, learning, commerce, community, assistant）
        ├── go.mod            独立依赖声明与版本锁定
        ├── go.sum
        ├── cmd/
        │   └── server/
        │       └── main.go   服务启动入口（组装配置、路由并启动服务，处理优雅退出）
        ├── api/              对外公开契约目录（Swaggo 自动生成 swagger.json 与 docs.go）
        ├── internal/         内部核心业务实现（直属 internal，拒绝深层嵌套套娃）
        │   ├── config/       服务配置（读取环境与启动校验）
        │   │   └── config.go
        │   ├── server/       路由注册与 HTTP Server 封装
        │   │   └── server.go
        │   └── <业务特性包>/  按业务特性命名（如 accounts, sessions, orders, courses 等）
        │       ├── <业务名>.go核心业务逻辑与数据模型（直接直观，无冗余多余包装）
        │       ├── handler.goHTTP 接口处理（参数绑定、响应与 Swaggo 注释）
        │       └── store.go  数据库持久化操作（Postgres SQL 操作）
        ├── tests/            独立测试目录（属于本服务 Go module，与 internal 平级）
        │   ├── integration/  服务 HTTP 接口与存储集成测试
        │   └── testdata/     按需添加的测试数据
        ├── Dockerfile        轻量独立生产镜像
        ├── Makefile          独立检查与生成入口
        └── README.md         服务说明与环境变量文档
```

每个微服务的组织特点：
- **扁平自包含**：打开任意业务特性包（例如 `accounts/`），即可一眼看清核心逻辑（`account.go`）、网络处理（`handler.go`）与存储访问（`store.go`），不需要跨多个层次目录来回跳转。
- **杜绝空接口与过度转接**：直接使用具体结构体与显式传参，除非存在跨包解耦或单测 Mock 诉求，不人为制造一层空 Interface。
- **契约对外公开**：文档与 OpenAPI 产物位于服务直属的 `api/`（由工具自动生成），不放入私有的 `internal/` 目录中。
- **入口负责组装**：`main.go` 负责环境校验与生命周期，`server.go` 负责注册路由，业务函数显式接收数据和 `context.Context`，不扩散框架上下文（如 `*gin.Context`）。

### 服务通信与数据所有权

- 对外经 Traefik 路由至目标服务；内部调用使用部署环境的服务 DNS 和 HTTP 客户端，传播超时、取消与追踪上下文，不硬编码 IP。
- 服务只访问自己拥有的数据。持久化默认采用 PostgreSQL，每个服务使用独立逻辑数据库和访问账号；本地可复用数据库实例，但不跨服务联表或共享表。数据库与权限边界参考 [PostgreSQL 文档](https://www.postgresql.org/docs/current/ddl-schemas.html)。
- 强一致事务在拥有数据的服务内部完成。跨服务流程在业务设计中明确失败、补偿和幂等要求，不伪装成一个跨库本地事务。
- 不因微服务架构默认添加分布式事务框架或事件总线。确需异步流程时再选定投递、去重、重试与恢复机制。
- 服务可独立构建、检查、部署及回滚；API 演进保持已有消费者兼容，跨服务变更明确发布顺序。

### 实现约定

- 规范基线为 Google Go，具体命名、包职责、接口、错误、context 与并发要求统一引用[Go 代码设计](./workspace/content/standards/001-项目文档与设计规范.md#go-代码设计)；Effective Go 作为补充资料。
- 接口在消费方确有需要时定义，保持最小能力；不为每个结构体或生成客户端配套包装接口。
- 请求的超时与取消向存储和外部调用传递；显式映射 HTTP 错误，使用结构化日志关联服务名、请求与 trace ID。OpenTelemetry 接入请求入口及跨服务调用。
- 启动时校验必需配置，处理优雅退出；探活用于进程状态，就绪检查依据实际必需依赖定义。
- 权限校验覆盖每次受保护的业务操作与资源访问，不能只依赖前端隐藏入口。
- 数据模型、SQL、事务范围和并发约束由实际业务设计决定。引入持久化时明确结构定义、变更与恢复方式，不先预设业务表或真实环境变更。

## 5. Swagger 与前端调用代码自动生成

### 契约自动生成铁律（严禁手动维护 API 文件）

**核心铁律：禁止手动编写、修改或维护任何 Swagger / OpenAPI 契约文件（包括 swagger.json、docs.go、YAML 文件等），严禁手写前端接口函数与 DTO 副本。**

- **唯一事实源（Source of Truth）**：接口契约必须且仅能通过在 Go 业务源码中维护 **Handler Swaggo 注解**（`@Summary`、`@Tags`、`@Param`、`@Success`、`@Failure`、`@Router` 等）以及 **DTO 结构体字段标签**（`json:"xxx" binding:"required" example:"xxx"`）来表达。
- **全自动代码生成流水线**：
  ```text
  Go Handler + DTO 字段标签 + Swaggo 注解
      → Swaggo CLI (swag init)：全自动编译生成各微服务的 api/ 契约文件 (swagger.json, docs.go)
      → gin-swagger：运行时 Swagger UI 预览与调试
      → @umijs/openapi (openapi2ts CLI)：根据当前 Swagger JSON 自动生成前端 TypeScript 类型与调用函数
      → Axios 适配器：通用请求执行
      → Next.js 页面与业务组件消费
  ```
- **禁止手动干预**：文档产物与前端生成的 `services/generated/` 均纳入忽略规则，由流水线从源码重建，任何接口变更一律直接修改 Go 源码中的注解与 DTO，重新执行生成命令即可生效。

Swaggo 按[官方说明](https://github.com/swaggo/swag)从 Go 注释生成 Swagger 2.0；展示使用 [gin-swagger](https://github.com/swaggo/gin-swagger)。这里明确采用 Swagger 2.0，不将其标记为 OpenAPI 3.x。注释仍需维护操作说明、路由、参数与状态信息，生成工具不能推断全部业务语义；通过路由覆盖和接口测试检查漂移。

各服务只生成自己的文档。开发环境提供 Swagger UI 与 JSON 地址，经网关按服务映射；生产环境按配置关闭或限制访问。前端按服务分别生成客户端、设置命名空间，不手写聚合 Swagger 文件。每个操作使用稳定且唯一的 `operationId`，避免重新生成后调用名无故变化。

### Next.js 接入 Umi OpenAPI 与 Axios

使用独立开发依赖 `@umijs/openapi`，不引入 Umi 应用框架或 `@umijs/plugin-openapi`。其[上游文档](https://github.com/chenshuai2144/openapi2typescript)支持 Swagger 2.0 / OpenAPI 3.0 输入和请求方法配置；本次同时核对了已发布包 `@umijs/openapi@1.14.1` 的生成模板，最终安装版本仍需在初始化时锁定。

`openapi2ts.config.ts` 只配置实际依赖的服务、生成文档来源、输出目录、命名空间和请求导入，不定义业务接口。通过官方 `requestImportStatement` 配置从 `lib/request/transport.ts` 导入请求方法与选项类型；生成代码使用现有 Axios 适配器。

接入采用上游推荐的独立 CLI：`package.json` 中声明 `"openapi2ts": "openapi2ts"`，在前端根目录执行 `pnpm openapi2ts`。Go 文档生成由后端负责，前端开发、构建、类型检查各自运行；首次使用或接口变更后先生成客户端。仅保留官方配置文件，不维护 `scripts/` 目录或独立生成脚本。

适配器只处理生成器与 Axios 的技术差异：接收生成调用的 URL、method、params、data 等选项，转换适用的请求选项，返回响应体 `response.data`，并保留 HTTP 错误、取消及必要响应元数据的获取方式。[Axios 请求配置](https://axios-http.com/docs/req_config)作为实现依据。文件、表单、下载和空响应按真实接口验证；没有此类接口时不预造处理框架。

浏览器使用同源网关地址，服务端使用配置的内部网关地址；二者复用生成的业务调用定义。SSR 请求的凭据按请求上下文显式传递，禁止把某个用户的 Cookie 或令牌写入进程共享 Axios defaults / 拦截器。服务端配置不得进入客户端包，API 凭据传递也不得隐藏在生成代码中。

### 生成与一致性检查

生成器和格式化工具使用固定版本。Swagger 产物和 `frontend/src/services/generated/` 不手改、不提交，纳入忽略规则；它们是由源码重建的构建产物，和知识库正文归属不同。

后端流水线从目标提交生成 Swagger 并构建服务，保存带服务名与源码版本的文档产物。前端使用本次指定后端版本的文档产物生成客户端，再执行类型、构建和接口兼容检查；CI 不从开发者本机或未锁定版本的在线服务抓取文档。仅发布前端时，可使用已发布且明确版本的文档产物，无需启动 Go 服务。

检查至少包括：公开业务路由的文档覆盖、方法和路径一致、唯一操作 ID、输入输出与错误响应、生成结果可重复，以及生成客户端的实际调用。类型通过只能证明静态匹配，仍需测试后端真实响应。后端使用 `backend/Makefile` 的 `generate`，前端使用官方 `openapi2ts` CLI；健康检查链路已完成实际生成和请求验证，后续业务接口仍需各自验证。

## 6. 访问、认证与配置

| 调用方 | 访问路径 | 职责 |
| --- | --- | --- |
| 浏览器 | Next.js 页面及必要的浏览器接口请求 | 展示页面、提交用户操作 |
| Next.js 服务端 | 配置的内部网关入口 | 使用生成的调用代码读取数据及适用的会话适配 |
| Traefik | 按服务路由至 Go API | TLS 与入口转发；不执行业务逻辑 |
| Go 服务 | 本服务数据库、其他服务 API 或必要外部依赖 | 执行权限与业务规则，维护本服务数据 |
| 知识库访问者 | workspace 静态网站 | 阅读知识；无需调用业务 API |

业务接口规划采用 `/api/<服务名>/` 路径，经 Traefik 保留前缀转发至对应 Gin 路由；Swagger 路径使用相同外部路径，不在生成器中再次拼接同一前缀。服务名与部署地址来自实际配置，当前健康检查与账号接口已按此路径落地，后续接口继续检查与 Next.js 自身端点的路径冲突。

契约至少明确字段类型、必填和范围约束、身份与资源权限、成功状态、错误状态与分页语义。保留 HTTP 状态语义；错误信息可供用户恢复并能定位请求，不暴露内部堆栈。只有接口确实返回总数时才展示总数。

认证供应方及会话方式随业务确定，各服务执行访问令牌或会话的有效性与资源权限校验。内部网络不代表可信身份；服务间调用还需定义调用方身份及授权，不信任浏览器可伪造的用户 ID 请求头。采用 Cookie 时设计 CSRF 防护和 Cookie 属性，跨域时显式配置允许来源与凭据策略。幂等、重试与并发控制针对实际操作设计，避免自动重试产生重复写入。

服务地址、端口、超时与允许来源通过模块配置管理，不写死在业务组件中。客户端只接收可公开配置；数据库凭据、服务密钥和内部访问地址保留在服务端。配置示例仅包含变量说明和安全占位值。

## 7. 开发、检查与部署

### 本地端口分配

微服务按业务域独立分配固定本地运行端口，统一入口由 Traefik 网关（`3121`）分发：

| 模块 / 服务 | 固定端口 | 服务职责与启动说明 |
| --- | --- | --- |
| **frontend** | `3121` | 原生 `pnpm dev` / `pnpm start`；容器组合由 Traefik 在同一端口提供统一入口 |
| **workspace** | `3122` | Fumadocs 知识库站点，`pnpm dev` 或构建后 `serve out --listen 3122` |
| **identity** | `3123` | 账号、认证与访问控制微服务 |
| **learning** | `3124` | 课程与媒体学习微服务 |
| **commerce** | `3125` | 交易、退款与权益微服务 |
| **community** | `3126` | 社区互动与内容治理微服务 |
| **assistant** | `3127` | 通用 Agent、知识库与 AI 治理微服务 |
| **studio** | `3128` | 原创 IP 单体试点，Compose 的 `studio` profile 或独立启动 |

原生前端和容器统一入口共用 3121，按需选择一种启动方式。前端容器内部监听 3121，各 Go 微服务内外使用各自独立端口（3123～3127）；API 和 Swagger 可通过统一入口或对应后端端口直接访问。端口仅在启动配置中指定，业务组件通过同源路径或服务端配置访问 API。部署环境可使用不同的网络映射，本表规定本项目本地运行端口。

业务服务各自产出镜像，本地通过 Docker Compose 联调 Traefik、前端、业务服务与必需依赖，也支持开发工具分别启动。生产使用独立服务部署，目标平台与高可用配置待运行需求明确后确定；Compose 是本地联调选型，不作为已完成的生产部署。

### 独立测试目录

```text
backend/services/<服务名>/tests/
├── unit/<业务包>/             导入本服务业务包，测试其导出行为
├── integration/              服务 HTTP、数据库与适用外部依赖的集成测试
└── testdata/                 按需添加的测试数据
frontend/tests/               前端单元、组件与模块集成测试
tests/
├── contracts/                跨服务及前后端契约验证
└── e2e/                      经真实入口执行的用户流程
```

服务内 `tests/` 与 `internal/` 是同级目录，仍属于同一个 Go module，可导入本服务 `internal` 包的导出能力；测试文件使用 `*_test.go`，按黑盒方式验证，不为访问私有实现额外导出测试专用符号。此组织遵守 [Go internal 可见性](https://go.dev/doc/go1.4#internalpackages)与[测试包规则](https://pkg.go.dev/testing)，是项目选定的测试布局，不声称 Go 官方要求将测试全部移出实现目录。

根 `tests/` 负责跨模块验证，通过实际接口或浏览器访问业务服务，不导入服务私有包。测试执行入口、数据与报告独立管理；测试所需容器和数据库只在对应测试环境启用，报告和运行数据不提交，也不包含在业务发布产物中。测试目录按实际测试创建，不预建空测试服务。

| 模块 | 实施后需要具备的检查 | 部署产物与边界 |
| --- | --- | --- |
| `frontend/` | 生成客户端、Prettier 检查、ESLint、类型检查、生产构建；实际接口及桌面、移动端、键盘操作验收 | Next.js 服务端部署，独立镜像 |
| 每个 Go 服务 | Swagger 生成与覆盖检查、格式检查、`go vet ./...`、`go test ./tests/...`、`go build ./cmd/server`；数据与权限集成测试 | 独立 Go 可执行程序与镜像；发布检查使用 `GOWORK=off` |
| 根 `tests/` | 跨模块契约与端到端验证，检查各服务兼容性及关键流程 | 独立测试执行入口与报告，不进入业务镜像 |
| `workspace/` | 沿用现有 lint、类型检查、构建及适用页面验收 | 现有静态导出目录 `workspace/out/` |

业务前端默认规划为支持服务端渲染的部署，以容纳服务端数据读取和会话适配；不沿用知识库的静态导出配置。[Next.js 官方部署说明](https://nextjs.org/docs/app/getting-started/deploying)区分服务端与静态部署能力，最终方式随业务和平台决定。

流水线按受影响服务执行检查，接口变更检查所有实际消费者。验证跨服务超时、依赖不可用及权限失败路径，记录 trace 与请求定位结果；模拟响应只作为局部验证。发布记录各服务版本、配置、探活与关键路径结果，涉及数据变更时说明兼容与恢复安排。当前启动与检查命令已记录在模块 README，后续业务能力更新相应说明。

服务测试需要覆盖业务包时显式指定 `-coverpkg=./internal/...`，例如规划命令 `go test -coverpkg=./internal/... ./tests/...`；独立测试包的默认覆盖率不能作为业务源码覆盖率。前端测试脚本只发现 `frontend/tests/`，跨模块测试由根 `tests/` 单独执行。

## 8. 落地顺序与交付条件

下表用于评审新增或改造的业务能力。初始化证据位于[工程初始化记录](./workspace/content/plan/001-前后端工程初始化计划与验证.md)。原创 IP 首版的具体任务，在需求、设计及 PRD 确认和实施获授权后通过[计划 Skill](./.agents/skills/workspace-plan/SKILL.md)拆解。

| 顺序 | 交付内容 | 进入下一步的条件 |
| --- | --- | --- |
| 1 | 明确用户、范围、功能需求和非功能需求，评审 PRD | 能说明完整流程、质量约束和可观察结果 |
| 2 | 按业务域确定服务清单、调用关系、数据归属和认证，完成前后端设计 | 每个服务的边界、接口和错误行为可供实施 |
| 3 | 核对现有入口，实施本次所需服务与前端变更 | 各服务独立构建；Go 源码能生成文档，再生成可编译的 Axios 调用代码 |
| 4 | 完成一条页面到目标服务与必要存储的业务流程，并验证实际跨服务调用 | Swagger UI 可用，生成客户端真实调用成功，正常、失败与权限路径通过验证 |
| 5 | 验证部署与交接 | 配置、产物、验收证据及恢复步骤可复核；上线另按实际授权执行 |

## 9. 设计决定与待确认项

| ID | 决定 | 依据与状态 |
| --- | --- | --- |
| D-01 | Next.js + TS + React + Tailwind CSS + shadcn/ui / Radix UI + Prettier + ESLint + Axios + Umi OpenAPI；后端使用 Go | 用户明确指定；原文 `aixos` 按 Axios 处理，已初始化 |
| D-02 | 三个模块位于根目录下，各自维护依赖与部署边界 | 本次工程设计；保持知识库已有独立能力 |
| D-03 | Go 按业务域采用微服务，每个服务独立模块、数据所有权和部署生命周期 | 用户明确要求微服务；现有五个服务与新制作域的候选边界分别记录 |
| D-04 | Go 为业务与数据权威，Next.js 负责页面和必要适配 | 设计建议，避免两端维护重复业务规则 |
| D-05 | Go 源码与注释 → Swaggo → Swagger UI / Umi OpenAPI → Axios 调用 | 用户要求文档与调用代码自动生成；选型已核对上游能力，健康检查已验证生成与工程集成；业务接口仍按需增加 |
| D-06 | Gin、Traefik、Compose DNS、PostgreSQL / pgx、slog / OpenTelemetry | 本次技术选型；按上文职责接入，不增加无业务依据的基础设施 |
| D-07 | Go 采用 Google Go，前端采用 Google TS；框架与生成代码仅保留明确范围的必要例外 | 用户要求按此标准规范文档与目录；规范已写入，工程检查已建立，具体范围见模块 README |
| D-08 | 项目标识统一为 `ai2workspace`，测试代码采用独立目录 | 用户本次明确要求，测试目录和执行边界按本文件定义 |

原创 IP 首版待评审的事项包括制作域部署边界、素材和媒体服务、真实支付环境、兼容迁移和质量目标。业务前端视觉规范见 [DESIGN.md](./DESIGN.md)。业务交付以对应 PRD、验收记录和发布记录为准。

业务范围、单机故障容忍度与生产高可用要求仍待确认，后续服务边界和部署设计以相应决定为依据。
