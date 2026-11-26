# ai2workspace backend

单一 Go 应用。入口位于 `cmd/ai2workspace/`，业务实现与测试目录见[工程目录](../workspace/02-项目工程说明.md#目录)。

## 本地启动

在仓库根目录执行：

```sh
docker compose up --build -d frontend
```

Compose 启动 PostgreSQL、后端和前端。应用入口为 `http://127.0.0.1:3121`，后端就绪检查为 `http://127.0.0.1:3123/api/health`，Swagger 为 `http://127.0.0.1:3123/swagger/index.html`。

`schema.sql` 是数据库结构的唯一事实源。Compose 在数据库卷首次初始化时应用它。原生运行时，在本目录将 `DATABASE_URL` 指向一个空 PostgreSQL 数据库并执行：

```sh
make init-db
SWAGGER_ENABLED=true make run
```

`HTTP_ADDR` 默认为 `127.0.0.1:3123`。按需同时设置 `BOOTSTRAP_ADMIN_EMAIL` 与 `BOOTSTRAP_ADMIN_PASSWORD` 创建本地管理员。涉及付费回调或媒体签发时，通过 `PAYMENT_WEBHOOK_SECRET` 和 `PLAYBACK_SIGNING_SECRET` 提供对应密钥。

## 生成与验证

```sh
make generate
make check build
```

`make generate` 生成一份 Swagger。进程启动依赖已初始化的数据库。用 `TEST_DATABASE_URL` 指向隔离且已初始化的数据库，可运行 `tests/` 中的数据库集成验收。
