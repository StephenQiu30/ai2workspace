# ai2workspace 前端

由官方 `create-next-app@16.4.6` 初始化，使用 Next.js、React、TypeScript strict、Tailwind CSS 和通过官方 CLI 引入的 shadcn/ui Radix 组件。首页只验证 ai2workspace 服务连接，不包含业务功能或示例业务数据。

使用 Node.js 24.19.0、pnpm 12.4.2，在本目录执行：

```sh
pnpm install --frozen-lockfile
pnpm openapi2ts
API_ORIGIN=http://127.0.0.1:3123 pnpm dev --hostname 127.0.0.1
```

先按 [后端说明](../backend/README.md) 在 `backend/` 执行 `make generate` 并启动 Go 服务，再运行上述前端命令，浏览器访问 `http://127.0.0.1:3121`。本地开发的 `/api/` 通过 Next.js rewrites 转发到 `API_ORIGIN`；容器组合由 Traefik 提供统一入口。

## 生成、检查与构建

```sh
pnpm openapi2ts
pnpm format:check
pnpm lint
pnpm types:check
pnpm test
pnpm build
```

按 [Umi OpenAPI 官方独立接入方式](https://github.com/chenshuai2144/openapi2typescript#usage)，根配置为 `openapi2ts.config.ts`，`package.json` 使用 `"openapi2ts": "openapi2ts"`，由官方 CLI 自动读取配置并生成 `src/services/generated/`。生成代码忽略提交，禁止手改。

默认读取本仓库 Go 服务生成的本地 Swagger 产物，文档生成由后端自行负责。也可以显式指定 Swagger JSON 地址或版本对应的绝对文件路径：

```sh
OPENAPI_SCHEMA_PATH=http://127.0.0.1:3123/swagger/ai2workspace/doc.json pnpm openapi2ts
OPENAPI_SCHEMA_PATH=/absolute/path/to/swagger.json pnpm openapi2ts
```

CLI 读取进程环境变量，独立于 Next.js 的 `.env` 加载。只有前端源码时，使用已发布的接口文档即可生成，无需安装 Go。接口生成与 dev、build、类型检查分开；首次使用或接口更新时先生成，再检查类型和构建。Docker 构建通过官方 CLI 使用 schema 阶段的产物完成生成。项目没有自建脚本目录。

`src/lib/request/transport.ts` 是唯一通用 Axios 适配入口；`server.ts` 使用 `server-only` 保护内部配置。服务端通过调用选项传入地址，浏览器通过同源路径请求，不修改全局 Axios 配置。当前接口无认证需求，未实现用户会话转发。

`API_ORIGIN` 必须是无凭据、路径、查询或片段的 HTTP(S) origin。Next.js 的 rewrite 随构建配置确定，生产统一入口使用 [Compose 配置](../backend/compose.yaml)，不依赖构建时写入服务器地址。生产页面动态渲染，后端暂不可用时显示可重试状态。

手写业务文件使用 snake_case 与命名导出，Next.js 特殊入口及 CLI 组件保留官方约定；ESLint 和类型检查覆盖适用的 Google TS 要求，完整约束仍需设计复核。Prettier 独立运行，未整体关闭业务检查。

Axios 测试位于独立 `tests/`，使用本机临时 HTTP 服务验证响应体、错误、取消与并发调用隔离。跨模块桌面 / 移动端验收见 [根测试说明](../tests/README.md)。
