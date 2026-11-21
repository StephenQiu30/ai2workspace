# identity 服务

该 module 是账号与访问控制微服务，提供账号生命周期、邮箱验证、会话管理、讲师邀请、账号治理及内部会话解析接口，并通过 Swaggo 生成 Swagger 契约。

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
| `DATABASE_URL` | 空（内存存储） | PostgreSQL 连接串；配置后连接 `pgxpool` 并幂等初始化 `schema.sql` |
| `SERVICE_TOKEN` | `identity-service-secret` | 内部服务鉴权令牌（`X-Service-Token`） |
| `COOKIE_SECURE` | `false` | 会话 Cookie `ai2_session` 的 `Secure` 属性（始终启用 `HttpOnly`、`SameSite=Lax`） |
| `DEV_EXPOSE_TOKENS` | `false` | 仅限开发联调时在响应 JSON 暴露一次性验证/重置/邀请令牌，默认关闭 |
| `BOOTSTRAP_ADMIN_EMAIL` | 空 | 启动时幂等初始化的管理员邮箱（需与密码同时配置） |
| `BOOTSTRAP_ADMIN_PASSWORD` | 空 | 启动时幂等初始化的管理员密码（至少 8 位） |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USER` / `SMTP_PASSWORD` / `SMTP_FROM` | 空 / `587` | 配置 `SMTP_HOST` 后尝试真实 SMTP 发信，失败返回错误并标记 `mail_tasks` 失败 |
| `RATE_LIMIT_AUTH_MAX` / `RATE_LIMIT_MAIL_MAX` / `RATE_LIMIT_WINDOW` | `20` / `10` / `1m` | 登录/注册与邮件类接口滑动窗口限流配置 |

程序只读取进程环境，不自动读取 `.env`；`.env.example` 是公开配置示例。收到 SIGINT / SIGTERM 后关闭监听并等待已有请求，超时后强制关闭并释放数据库连接池。

`internal/` 保存服务业务实现，`api/` 保存 Swaggo 自动生成的公开契约，`tests/` 单独维护配置与 HTTP 集成测试。`api/` 和本机构建的 `server` 为忽略提交的生成物，严禁手动维护 Swagger 定义。
