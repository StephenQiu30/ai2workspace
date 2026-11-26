# ai2workspace 前端

使用 Next.js、React、TypeScript strict、Tailwind CSS 和通过官方 CLI 引入的 shadcn/ui Radix 组件。现有页面包括社区、课程、账号、订单和运营后台。原创 IP 首版设计见[知识库现行专题](../workspace/content/index.md#学屿原创-ip-制作与共创-saas)。

使用 Node.js 24.19.0、pnpm 12.4.2，在本目录执行：

```sh
pnpm install --frozen-lockfile
pnpm openapi2ts
pnpm dev --hostname 127.0.0.1
```

原生开发先按[后端说明](../backend/README.md)执行 `make generate` 并启动 Go 后端。默认访问 `http://127.0.0.1:3121`，Next.js 将同源 API 转发到本地 3123。

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

`openapi2ts.config.ts` 读取后端统一生成的 Swagger JSON。首次使用或接口更新时先由后端生成 Swagger，再生成前端调用并检查。Docker 构建通过官方 CLI 使用 schema 阶段的产物完成生成。

`src/lib/request/transport.ts` 提供通用 Axios 适配。`server.ts` 使用 `server-only` 保护内部配置，并按单次请求传递服务端地址及适用的登录会话。浏览器使用同源路径。

`API_ORIGIN` 如设置，须是无凭据、路径、查询或片段的 HTTP(S) origin。原生开发默认使用 `http://127.0.0.1:3123`。容器构建与运行使用 `http://backend:3123`。正式部署的外部地址和依赖以相应环境配置与验收为准。

手写业务文件使用 snake_case 与命名导出，Next.js 特殊入口及 CLI 组件保留官方约定；ESLint 和类型检查覆盖适用的 Google TS 要求，完整约束仍需设计复核。Prettier 独立运行，未整体关闭业务检查。

前端测试位于本模块的 `tests/`。跨模块验收按[项目工程规范](../PROJECT.md#独立测试目录)执行。
