# ai2workspace

独立模块：

- [workspace](./workspace/README.md)：Fumadocs 知识库，正文位于 `workspace/content/`。
- [frontend](./frontend/README.md)：官方 Next.js 脚手架与 shadcn/ui Radix，使用自动生成的 Axios 调用代码。
- [backend](./backend/README.md)：Go 微服务工程，当前服务基线为 `services/ai2workspace/`。
- [tests](./tests/README.md)：独立的契约与桌面、移动端端到端测试。

工程职责、Google Go / TS 规范与后续业务边界见 [PROJECT.md](./PROJECT.md)。当前只初始化健康检查链路，业务域、认证和数据模型仍待确定。

本地容器联调：

```sh
cd backend
docker compose up --build -d
```

默认访问 http://127.0.0.1:3121 ，Swagger 位于 `/swagger/ai2workspace/index.html`。模块 README 记录独立启动、配置、生成与检查命令。

知识库独立运行：

```sh
cd workspace
pnpm install --frozen-lockfile
pnpm dev
```

本地访问 http://localhost:3122/docs/ 。完整的[知识库使用说明](./workspace/content/index.md)由 workspace 承载。
