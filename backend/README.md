# ai2workspace backend

Go 服务按 `services/<服务名>/` 独立维护 module、入口、测试和镜像。当前只有 `services/ai2workspace/` 技术基线，提供健康检查和可选 Swagger；业务域尚未定义，没有引入数据库、认证或共享业务框架。

使用 Go 1.26.5，在本目录执行：

```sh
make generate
make check
make build
HTTP_ADDR=127.0.0.1:1152 SWAGGER_ENABLED=true make run
```

`generate` 固定使用 Swaggo 1.16.6，从 Go DTO 与注释生成文档。默认 `SERVICE=ai2workspace`；未来新增真实服务时按实际名称选择，不复制一套通用封装。

- 健康检查：`http://127.0.0.1:1152/api/ai2workspace/v1/health`
- 接口文档：`http://127.0.0.1:1152/swagger/ai2workspace/index.html`
- 配置与独立检查：[服务 README](./services/ai2workspace/README.md)

## 容器联调

在本目录执行：

```sh
docker compose up --build -d
docker compose ps
```

默认入口 `http://127.0.0.1:1150`，由 `AI2WORKSPACE_PORT` 配置。Traefik 使用只读文件配置与 Compose DNS 转发真实服务，不需要访问 Docker socket。`/api/ai2workspace/` 和 `/swagger/ai2workspace/` 转发到 Go，其他路径转发到前端。Swagger 默认仅在这个本地联调组合中开启，独立 Go 服务默认关闭。

镜像构建从 Go 源码生成 Swagger，再生成前端调用代码。镜像仅包含运行产物，测试与知识库不进入镜像。生产部署、认证、持久化与高可用仍需单独设计。
