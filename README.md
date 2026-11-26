# ai2workspace

独立模块：

- [workspace](./workspace/README.md)：Fumadocs 知识库，正文位于 `workspace/content/`。
- [frontend](./frontend/README.md)：官方 Next.js 脚手架与 shadcn/ui Radix，使用自动生成的 Axios 调用代码。
- [backend](./backend/README.md)：Go 微服务工程，包含身份、学习、交易、社区和 Agent 服务。
- 前端测试位于 `frontend/tests/`，Go 测试位于各服务的 `tests/`。

工程职责与 Google Go / TS 规范见 [PROJECT.md](./PROJECT.md)。原创 IP 首版的需求、设计与验收见[知识库现行专题](./workspace/content/index.md#学屿原创-ip-制作与共创-saas)。

在仓库根目录使用 [docker-compose.yml](./docker-compose.yml) 联调：

```sh
docker compose up --build -d
```

默认入口为 http://127.0.0.1:3121 。Swagger 按服务分发至 `/swagger/<服务名>/index.html`。模块 README 记录独立启动、配置、生成与检查命令。

知识库独立运行：

```sh
cd workspace
pnpm install --frozen-lockfile
pnpm dev
```

本地访问 http://localhost:3122/docs/ 。完整的[知识库使用说明](./workspace/content/index.md)由 workspace 承载。

## 开源协议

本项目基于 [MIT 协议](./LICENSE) 开源。
