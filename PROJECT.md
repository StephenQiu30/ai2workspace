# ai2workspace 工程配置

## 范围与布局

本机单用户、多空间项目工作区工具；Codex 发起任务，Core 管理知识身份、上下文、检查证据和交接事实，CLI 执行声明检查，Web 提供官方 Fumadocs 阅读与治理展示。第一版代码与目录已落地；本机验证及剩余验收见 [BACKLOG.md](./BACKLOG.md)及 [实施计划](./docs/development/02-最小闭环实施计划.md)。

| 路径                                                              | 职责                                                                                 |
| ----------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| `apps/web`                                                        | Next.js 16、React 19、Fumadocs Core/UI、官方搜索与 shadcn 主题                       |
| `apps/cli`                                                        | 薄 CLI，共享 Core 判定；Web 未启动也可使用                                           |
| `packages/core`                                                   | Workspace / Knowledge / Context / Harness 分域，独立类型与 API；不依赖 Next.js/React |
| `docs`                                                            | 服务自身的产品、架构、指南、开发和历史文档；用户知识保留在用户选择的位置             |
| `templates/workspace`、`templates/knowledge`、`templates/harness` | 按空间、知识和研发任务分开的可复制模板                                               |
| `spaces/self`                                                     | 自用空间共享配置；本机绑定和运行数据私有                                             |

Node.js 24、pnpm 12.4.2，根 pnpm workspace 统一安装依赖；精确版本以根及各包 `package.json`、根 `pnpm-lock.yaml` 为准。默认本机地址为 http://127.0.0.1:1140/，dev 与 start 择一占用端口，不停止其他项目进程。

## 命令

所有命令从仓库根执行：

```bash
pnpm install --frozen-lockfile
pnpm dev
pnpm build
pnpm start
pnpm build:cli
pnpm cli --workspace spaces/self space validate --json
pnpm cli --workspace spaces/self source scan --json
```

适用质量检查为 `pnpm lint`、`pnpm types:check`、`pnpm format:check`、`pnpm test`、`pnpm build`、`pnpm test:e2e`。pnpm test 包含 Core、CLI 和 Web 单测；浏览器验收单独执行。类型检查和单测先构建 Core，避免命名领域 API 使用旧声明。Playwright 默认启动正式构建预览，先构建；已有预览通过 `WORKSPACE_TEST_URL` 指定。端口覆盖使用 `pnpm --filter @ai2workspace/web exec next dev --hostname 127.0.0.1 --port <端口>`。

## 配置与运行边界

`workspaceDir/workspace.yaml` 可共享，只保存逻辑 binding；`workspace.local.yaml` 保存本机绝对路径和 dataDir，排除 Git。职责与文档归属见 [职责约定](./docs/architecture/03-职责与文档归属.md)。Core 内建 `generic` / `development` 两种 profile，docs/guides/profiles 中的说明文件不构成额外运行时配置。字段以 [contracts.ts](./packages/core/src/contracts.ts)和实际 schema 校验为准。

自用空间 `ai2workspace-dev`：knowledge-root 绑定 `docs`，service-root 绑定当前仓库；engineering-rules 显式选取 AGENTS、PROJECT、DESIGN。检查通过 argv 执行 `pnpm types:check`、`pnpm test`，只声明 node/pnpm 版本和公开环境，不依赖私密环境。必需产物仅接受本次运行根目录下的新文件；原始输出与运行数据不发布。

阅读与本机 CLI 不要求数据库、云服务或付费模型。首期不支持私有环境证明、报告导入或人工例外写入；合同要求此类条件时应报告阻塞。CLI/Skill 不调度 Codex，不提供网页执行接口，不自动提交、重置或发布。真实跨用户 Codex 会话验收只能由实际新会话提供。

TypeScript 保持 strict，遵循 Google TypeScript Style Guide；React/Next.js 使用官方建议与 Vercel React Best Practices，优先服务端组件，客户端只承载交互，跨边界数据最小化；大型模块按需加载，不无依据抽象。按风险测试核心逻辑，跨边界补适用集成验证；纯文档与低风险配置不机械加单测。如引入数据库，完整 schema.sql 为结构唯一来源，data.sql 为公共种子，不加增量 migrations 或运行时动态 DDL；新空库初始化和已有库受控变更分别处理。

部署使用标准 Next.js build/start 与 workspace 根安装，Web 包在 `apps/web`；具体托管、访问范围和远端发布仍按授权处理。本机验证不代表远端 CI 或生产发布。
