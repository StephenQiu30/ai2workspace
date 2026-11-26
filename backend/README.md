# ai2workspace backend

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
