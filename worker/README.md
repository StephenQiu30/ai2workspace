# ai2workspace worker

独立的 Node.js 24 / TypeScript Worker，连接 Temporal 的 `ai2workspace` 任务队列。当前的 `workerSmoke` 工作流供初始化验收使用。

在一个终端启动本地 Temporal：

```sh
temporal server start-dev
```

另开终端，在本目录执行：

```sh
pnpm install --frozen-lockfile
pnpm lint
pnpm types:check
pnpm format:check
pnpm build
pnpm start
```

Worker 默认连接 `127.0.0.1:7233`、`default` 命名空间，可用 `TEMPORAL_ADDRESS` 和 `TEMPORAL_NAMESPACE` 覆盖。从 `backend/` 执行以下命令，经 Go SDK 发起工作流并核对 Worker 的结果：

```sh
make studio-generate
TEMPORAL_TEST_ADDRESS=127.0.0.1:7233 GOWORK=off go test ./tests -run '^TestWorkerSmoke$' -v
```
