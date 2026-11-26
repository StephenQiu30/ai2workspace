# ai2workspace backend

## 原创 IP 单体试点

从仓库根目录启动 PostgreSQL、goose 迁移和 Go API。试点通过现有 identity 服务验证登录会话。

```sh
docker compose --profile studio up --build -d studio
```

- PostgreSQL 就绪检查：`http://127.0.0.1:3128/api/studio/health`
- Swagger UI：`http://127.0.0.1:3128/swagger/studio/index.html`
- 当前会话的工作区角色：`GET /api/studio/workspaces/{workspaceId}/access`

试点工作区由数据库预置。权限表以工作区和既有账号 ID 确定角色，支持 `owner` 和 `member`。设置 `revoked_at` 后，下次请求即拒绝访问。账号与会话继续由现有 identity 服务管理。

在隔离的开发数据库里，可用已登录账号从 `GET /api/identity/me` 获取 `userId`，再为试点预置一条成员记录。

```sh
docker compose --profile studio exec -T studio-db psql -U studio -d studio \
  -v ON_ERROR_STOP=1 -v user_id='替换为实际 userId' <<'SQL'
BEGIN;
INSERT INTO identity.workspaces (id) VALUES ('demo') ON CONFLICT DO NOTHING;
INSERT INTO identity.workspace_members (workspace_id, user_id, role)
VALUES ('demo', :'user_id', 'owner') ON CONFLICT DO NOTHING;
COMMIT;
SQL
```

使用同一会话请求 `/api/studio/workspaces/demo/access` 即可验证授权。

原生开发可单独启动 `docker compose --profile studio up -d studio-db identity`（在仓库根目录执行），随后在本目录使用以下命令。

```sh
export STUDIO_DATABASE_URL='postgres://studio:studio-local@127.0.0.1:54329/studio?sslmode=disable'
make studio-migrate
STUDIO_SWAGGER_ENABLED=true make studio-run
```

`STUDIO_HTTP_ADDR` 默认 `127.0.0.1:3128`，`STUDIO_IDENTITY_ORIGIN` 默认 `http://127.0.0.1:3123`。Swagger 和 Umi 客户端按现有规则从 Go 注解生成。

本目录的 `make studio-check studio-build` 执行静态检查、Go 测试和编译。要执行工作区授权集成测试，先迁移隔离 PostgreSQL，再传入 `STUDIO_TEST_DATABASE_URL`。Go 与 TS Worker 互通验证见 [Worker README](../worker/README.md)。

## 现有微服务

Go 微服务按 `services/<服务名>/` 独立维护 module、入口、测试和镜像。现有服务为 `identity`、`learning`、`commerce`、`community` 与 `assistant`。以下独立命令以 `identity` 为例。

使用 Go 1.26.5，在本目录执行：

```sh
make generate
make check
make build
HTTP_ADDR=127.0.0.1:3123 SWAGGER_ENABLED=true make run
```

`generate` 固定使用 Swaggo 1.16.6，从 Go DTO 与注释生成文档至 `api/` 目录。默认 `SERVICE=identity`；其他微服务按需指定 `SERVICE=<服务名>`。

- 健康检查：`http://127.0.0.1:3123/api/identity/health`
- 接口文档：`http://127.0.0.1:3123/swagger/identity/index.html`
- 配置与独立检查：[服务 README](./services/identity/README.md)

## 容器联调

在仓库根目录使用 [docker-compose.yml](../docker-compose.yml)：

```sh
docker compose up --build -d
docker compose ps
```

网关统一入口 `http://127.0.0.1:3121`。Traefik 按 `/api/<服务名>/` 和 `/swagger/<服务名>/` 转发到五个服务，其余页面交给前端。各服务直连端口见 [PROJECT.md](../PROJECT.md#本地端口分配)。

镜像构建从 Go 源码生成 Swagger，再生成前端调用代码。镜像仅包含运行产物。生产配置、外部依赖和高可用条件按部署环境及实际验收确认。
