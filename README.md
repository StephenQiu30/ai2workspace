# ai2workspace

知识库的网站代码和正文统一由 `workspace/` 承载，可独立构建与部署。

业务服务规划为 `frontend/`（Next.js、TypeScript、React、Tailwind CSS、shadcn/ui / Radix UI）和 `backend/`（Go 微服务），目前尚未初始化。Swagger 与前端调用代码自动生成，完整技术选型、服务边界及推进顺序见 [项目工程规划](./PROJECT.md)。

```sh
cd workspace
pnpm install --frozen-lockfile
pnpm dev
```

本地访问 http://localhost:3000/docs/ 。

- [模块入口](./workspace/README.md)
- [使用、编写与部署说明](./workspace/content/index.md)
