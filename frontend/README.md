# ai2workspace 前端

由官方 `create-next-app@16.4.6` 初始化，使用 Next.js、React、TypeScript strict、Tailwind CSS 和通过官方 CLI 引入的 shadcn/ui Radix 组件。首页只验证 ai2workspace 服务连接，不包含业务功能或示例业务数据。

使用 Node.js 24.19.0、pnpm 12.4.2，在本目录执行：

```sh
pnpm install --frozen-lockfile
API_ORIGIN=http://127.0.0.1:1152 pnpm dev --port 1151 --hostname 127.0.0.1
```

先按 [后端说明](../backend/README.md) 启动 Go 服务，浏览器访问 `http://127.0.0.1:1151`。本地开发的 `/api/` 通过 Next.js rewrites 转发到 `API_ORIGIN`；容器组合由 Traefik 提供统一入口。

## 生成、检查与构建

```sh
pnpm api:generate
pnpm format:check
pnpm lint
pnpm types:check
pnpm test
pnpm build
```

默认先执行 `make -C ../backend generate`，然后由固定版本 `@umijs/openapi@1.14.1` 生成 `src/services/generated/`。仅有前端源码时，通过 `OPENAPI_SCHEMA_PATH` 指定与目标后端版本对应的本地 Swagger 产物，无需运行后端进程或安装 Go。生成代码忽略提交，禁止手改；没有文档产物时命令失败，不使用过期类型静默回退。

`src/lib/request/transport.ts` 是唯一通用 Axios 适配入口；`server.ts` 使用 `server-only` 保护内部配置。服务端通过调用选项传入地址，浏览器通过同源路径请求，不修改全局 Axios 配置。当前接口无认证需求，未实现用户会话转发。

`API_ORIGIN` 必须是无凭据、路径、查询或片段的 HTTP(S) origin。Next.js 的 rewrite 随构建配置确定，生产统一入口使用 [Compose 配置](../backend/compose.yaml)，不依赖构建时写入服务器地址。生产页面动态渲染，后端暂不可用时显示可重试状态。

手写业务文件使用 snake_case 与命名导出，Next.js 特殊入口及 CLI 组件保留官方约定；ESLint 和类型检查覆盖适用的 Google TS 要求，完整约束仍需设计复核。Prettier 独立运行，未整体关闭业务检查。

Axios 测试位于独立 `tests/`，使用本机临时 HTTP 服务验证响应体、错误、取消与并发调用隔离。跨模块桌面 / 移动端验收见 [根测试说明](../tests/README.md)。
