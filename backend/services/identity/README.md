# identity 服务

该 module 是账号与访问控制微服务，提供 `GET /api/identity/health` 健康检查与 Swagger 契约。

使用 Go 1.26.5。先在 `backend/` 执行 `make generate`，然后可在本目录独立执行：

```sh
GOWORK=off go vet ./...
GOWORK=off go test -race -coverpkg=./internal/... ./tests/...
GOWORK=off go build -o server ./cmd/server
HTTP_ADDR=127.0.0.1:3123 SWAGGER_ENABLED=true ./server
```

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `HTTP_ADDR` | `:3123` | 监听地址，必须是有效的 host:port |
| `HTTP_TIMEOUT` | `5s` | 读取、写入及空闲超时，必须为正数 |
| `SHUTDOWN_TIMEOUT` | `10s` | 优雅退出等待时间，必须为正数 |
| `SWAGGER_ENABLED` | `false` | 开启 `/swagger/identity/` 文档与 JSON |

程序只读取进程环境，不自动读取 `.env`；`.env.example` 是公开配置示例。收到 SIGINT / SIGTERM 后关闭监听并等待已有请求，超时后强制关闭。

`internal/` 保存服务业务实现，`api/` 保存 Swaggo 自动生成的公开契约，`tests/` 单独维护配置与 HTTP 集成测试。`api/` 和本机构建的 `server` 为忽略提交的生成物，严禁手动维护 Swagger 定义。
