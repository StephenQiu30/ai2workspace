# ai2workspace 工程设计

学屿的业务前端、Go 后端和知识库分别位于 `frontend/`、`backend/` 和 `workspace/`。原创 IP 的产品范围和技术选型见[系统设计](./workspace/content/design/018-原创IP首版系统设计与验收总览.md)和[架构设计](./workspace/content/design/019-原创IP技术选型与架构重设计.md)。开发规范见[工程规范](./workspace/content/standards/001-项目文档与设计规范.md#工程规范与目录设计)，界面规范见 [DESIGN.md](./DESIGN.md)。

## 后端

`backend/` 是一个 Go module、一个进程和一个部署单元。`cmd/ai2workspace/main.go` 读取配置、建立 PostgreSQL 连接池并管理进程生命周期。`internal/app/` 装配业务域并统一挂载 Gin 路由。

| 模块 | 职责 |
| --- | --- |
| `internal/identity/` | 账号、会话和账号访问控制 |
| `internal/creation/` | 原创 IP 制作。当前提供工作区授权查询，剧本、媒体渲染、审片与发布按专题实现 |
| `internal/agent/` | Agent 执行、知识检索、模型调用和费用 |
| `internal/learning/` | 课程、媒体和学习进度 |
| `internal/commerce/` | 订单、支付、权益和退款 |
| `internal/community/` | 讨论、互动、通知和治理 |
| `internal/infra/` | 进程配置、PostgreSQL 连接与启动校验 |
| `internal/app/` | 统一 HTTP 路由与模块依赖装配 |

运行和异步执行归业务模块。Agent 步骤属于 `agent`，制作渲染属于 `creation`，课程媒体处理属于 `learning`。模块按需引入工具及调度依赖。编译产物由 Go 或镜像构建生成。

### 目录

```text
backend/
├── cmd/ai2workspace/   命令入口和进程生命周期
├── internal/           单体实现
│   ├── app/            HTTP 路由和模块装配
│   ├── infra/          进程配置和数据库连接
│   └── <业务域>/
│       ├── module.go  域内依赖与模块入口
│       ├── adapter/   HTTP、PostgreSQL 和外部服务适配
│       ├── application/  用例、任务和业务接口
│       └── domain/    业务模型和规则
├── tests/              按业务域归档，app/ 验证整应用
├── api/                生成的 Swagger
├── schema.sql         数据库结构定义
└── go.mod
```

文件分工、跨域依赖和测试导入边界见[单体模块的目录与依赖](./workspace/content/standards/001-项目文档与设计规范.md#单体模块的目录与依赖)。

### 数据和 API

`backend/schema.sql` 是 PostgreSQL 表和索引的定义。Compose 首次创建空数据库时加载它，原生环境在空库执行 `make init-db`。更新结构直接修改该文件。应用启动时连接已初始化的数据库，所有模块共享同一个连接池。

Go DTO 与 Swaggo 注解定义接口。运行 `make -C backend generate` 生成单一 Swagger，再从 `frontend/` 执行 `pnpm openapi2ts` 生成 Axios 客户端。生成物按项目约定忽略提交。业务 URL 前缀用于区分应用内的模块。

## 前端与知识库

业务前端使用 Next.js App Router、React、TypeScript strict、Tailwind CSS 和 shadcn/ui Radix。页面组合优先复用现有布局和组件。浏览器使用同源 `/api/`，服务端按单次请求向 `API_ORIGIN` 转发适用会话。知识库由 Fumadocs 独立构建和运行。

### 本地端口分配

| 工程 | 端口 | 用途 |
| --- | --- | --- |
| `frontend` | 3121 | 业务入口 |
| `workspace` | 3122 | 知识库 |
| `backend` | 3123 | 统一 HTTP API、健康检查和 Swagger |

从根目录用 Docker Compose 启动。具体命令见[后端](./backend/README.md)和[前端](./frontend/README.md)。

### 独立测试目录

- `backend/tests/<业务域>/` 保存该域的测试，`backend/tests/app/` 核对依赖方向、整应用路由和工作区授权。工作区授权集成测试需要已初始化的隔离 PostgreSQL。
- `frontend/tests/` 保存前端模块测试。
- `tests/` 按需保存跨模块契约与端到端验收。
