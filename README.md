# ai2workspace

- [frontend](./frontend/README.md) 提供学屿业务界面。
- [backend](./backend/README.md) 是模块化 Go 单体，使用一套 PostgreSQL 和一份 Swagger。
- [workspace](./workspace/README.md) 是 Fumadocs 知识库，正文位于 `workspace/content/`。

从仓库根目录启动应用：

```sh
docker compose up --build -d frontend
```

业务入口为 `http://127.0.0.1:3121`。知识库可单独运行 `docker compose up --build -d workspace`，访问 `http://127.0.0.1:3122/docs/`。

工程职责见 [项目工程说明](./workspace/02-项目工程说明.md)，首版产品见[原创 IP 专题](./workspace/content/00-文档导航.md#学屿原创-ip-制作与共创-saas)。

## 开源协议

本项目基于 [MIT 协议](./LICENSE) 开源。
