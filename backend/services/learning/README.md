# learning 服务

该 module 是课程与媒体学习微服务，提供 `GET /api/learning/health` 健康检查、课程供给、发布管理与可售快照 Swagger 契约。

使用 Go 1.26.5。先在 `backend/` 执行 `make generate SERVICE=learning`，然后可在本目录独立执行：

```sh
GOWORK=off go vet ./...
GOWORK=off go test -race -coverpkg=./internal/... ./tests/...
GOWORK=off go build -o server ./cmd/server
HTTP_ADDR=127.0.0.1:3124 SWAGGER_ENABLED=true ./server
```

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `HTTP_ADDR` | `:3124` | 监听地址，必须是有效的 host:port |
| `HTTP_TIMEOUT` | `5s` | 读取、写入及空闲超时，必须为正数 |
| `SHUTDOWN_TIMEOUT` | `10s` | 优雅退出等待时间，必须为正数 |
| `SWAGGER_ENABLED` | `false` | 开启 `/swagger/learning/` 文档与 JSON |
| `SERVICE_TOKEN` | `identity-service-secret` | 内部服务鉴权令牌 |
