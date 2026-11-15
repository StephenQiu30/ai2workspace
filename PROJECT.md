# Ai2sd 工程配置

## 当前范围

本项目当前交付研发知识工作区。业务方向、业务前后端、数据库、供应商与部署环境待需求确认；目录只创建当前实际使用的内容。

## 工作区

| 项目       | 配置                                                     |
| ---------- | -------------------------------------------------------- |
| 工程目录   | workspace/，独立安装依赖                                 |
| 运行时     | Node.js 24，pnpm 12.4.2                                  |
| 框架       | Next.js 16、React 19、TypeScript strict                  |
| 文档       | Fumadocs Core / UI 16.19.3、MDX 15.6.2                   |
| 组件与主题 | 官方 Base UI，shadcn Base Nova / Neutral，Tailwind CSS 4 |
| 搜索       | Fumadocs 内置同源 /api/search，多语言分词                |
| 内容       | 标准 Markdown，稳定文档 ID，Mermaid 按需加载             |
| 本机入口   | http://127.0.0.1:1140/，免登录阅读                       |

精确依赖以 workspace/package.json 和 pnpm-lock.yaml 为准。参考 Glowloom 当前工作区的配置，不带入其业务、品牌、凭据、历史兼容路由或外部基础设施。

## 命令

首次执行 `pnpm --dir workspace install --frozen-lockfile`，之后可在根目录执行 `pnpm dev`、`pnpm build`、`pnpm start`。

质量检查：`pnpm lint`、`pnpm types:check`、`pnpm format:check`、`pnpm build`、`pnpm test:e2e`。浏览器测试默认自行启动本项目的正式构建预览；设置 WORKSPACE_TEST_URL 时使用已有站点。详见 workspace/README.md。

## 配置与边界

初始工作区无需环境变量、数据库、Meilisearch 或云服务。需要改变端口时使用 `pnpm --dir workspace exec next dev --hostname 127.0.0.1 --port <端口>`，浏览器测试同时设置 WORKSPACE_TEST_URL。默认端口 1140 避免占用 Glowloom 的 1130。

根目录、workspace 和规范文档的职责见 README.md 与 AGENTS.md。部署采用标准 Next.js build/start；本机检查不代表远端 CI 或生产发布。
