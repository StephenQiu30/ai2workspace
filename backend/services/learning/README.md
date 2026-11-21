# learning 服务

该 module 是课程与媒体学习微服务，提供课程目录与搜索分类、讲师编排与管理员审核发布、视频直传校验与受控播放、学习进度与继续学习，以及面向 `commerce` / `community` / `assistant` 的内部访问与交付证据契约。

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
| `DATABASE_URL` | 空（内存 Store） | PostgreSQL 连接串；配置后使用 `pgxpool` 持久化并自动初始化 `schema.sql` |
| `SERVICE_TOKEN` | `identity-service-secret` | 内部服务鉴权令牌（`X-Service-Token`） |
| `IDENTITY_SERVICE_URL` | 空（拒绝会话） | `identity` 服务地址，用于解析 `ai2_session` Cookie |
| `COMMERCE_SERVICE_URL` | 空（无付费权益） | `commerce` 服务地址，用于校验单课购买权益及合并“我的课程” |
| `PLAYBACK_SIGNING_SECRET` | 同 `SERVICE_TOKEN` | HLS 播放令牌 HMAC-SHA256 签名密钥 |
