# ai2workspace backend

Go 微服务按 `services/<服务名>/` 独立维护 module、入口、测试和镜像。首个业务微服务为 `services/identity/`，提供身份与访问控制基础能力、健康检查与公开 Swagger 契约。

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

在仓库根目录执行：

```sh
docker compose up --build -d
docker compose ps
```

网关统一入口 `http://127.0.0.1:3121`，`identity` 服务也可通过 `http://127.0.0.1:3123` 直接访问。Traefik 负责将 `/api/identity/` 和 `/swagger/identity/` 转发到 `identity` 服务，其他路径转发到前端。中间件环境全部直连宿主机本机环境（`host.docker.internal`）。

镜像构建从 Go 源码生成 Swagger，再生成前端调用代码。镜像仅包含运行产物，测试与知识库不进入镜像。生产部署、认证、持久化与高可用仍需单独设计。
