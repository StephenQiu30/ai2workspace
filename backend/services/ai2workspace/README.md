# ai2workspace 服务

该 module 是独立 Go 服务基线，仅提供 `GET /api/ai2workspace/v1/health`，返回服务标识与运行状态。没有外部依赖，因此当前健康检查只代表本进程可响应，不代表未来数据库或其他服务可用。

使用 Go 1.26.5。先在 `backend/` 执行 `make generate`，然后可在本目录独立执行：

```sh
GOWORK=off go vet ./...
GOWORK=off go test -race -coverpkg=./internal/... ./tests/...
GOWORK=off go build -o server ./cmd/server
HTTP_ADDR=127.0.0.1:1152 SWAGGER_ENABLED=true ./server
```

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | 监听地址，必须是有效的 host:port |
| `HTTP_TIMEOUT` | `5s` | 读取、写入及空闲超时，必须为正数 |
| `SHUTDOWN_TIMEOUT` | `10s` | 优雅退出等待时间，必须为正数 |
| `SWAGGER_ENABLED` | `false` | 开启 `/swagger/ai2workspace/` 文档与 JSON |

程序只读取进程环境，不自动读取 `.env`；`.env.example` 是公开配置示例。收到 SIGINT / SIGTERM 后关闭监听并等待已有请求，超时后强制关闭。

`internal/` 保存服务实现，`tests/` 单独维护配置与 HTTP 黑盒测试。`internal/apidocs/` 和本机构建的 `server` 为忽略提交的生成物，不手动维护 Swagger 定义。
