# ai2workspace

本机单用户、多空间的项目工作区工具。知识治理提供可信资料，上下文组织提供显式选择与缺口，Harness 提供研发检查与交接。用户在 Codex 发起任务，共享 Core 提供知识接入、上下文快照、检查证据和交接恢复；CLI 执行显式配置的工程检查，Web 使用官方 Fumadocs 阅读和展示知识。产品方向与实施授权已确认，实现范围、验证证据和剩余验收见 [BACKLOG.md](./BACKLOG.md)。

## 启动

需要 Node.js 24 与 pnpm 12.4.2，在仓库根目录执行：

```bash
pnpm install --frozen-lockfile
pnpm setup:local
pnpm dev
```

本机阅读入口为 http://127.0.0.1:1140/。构建 CLI 后可显式指定空间：

```bash
pnpm build:cli
pnpm cli --workspace spaces/self space validate --json
pnpm cli --workspace spaces/self source scan --json
```

自用空间的共享配置在 [spaces/self/workspace.yaml](./spaces/self/workspace.yaml)；本机路径写入不入 Git 的 `workspace.local.yaml`。新空间参考 [templates/workspace](./templates/workspace/README.md)，每次都显式指定空间，不按当前目录猜测。

## 项目入口

- [协作入口](./AGENTS.md)、[工程配置](./PROJECT.md)、[视觉基线](./DESIGN.md)。
- [知识索引](./docs/00-文档索引.md)、[第一版设计](./docs/architecture/02-知识治理与Harness设计.md)、[实施计划](./docs/development/02-最小闭环实施计划.md)。
- [项目 Skill](.agents/skills/ai2workspace/SKILL.md)：合同、准备、检查、交接与恢复。
- `apps/web` 为阅读应用，`apps/cli` 为本机入口，`packages/core` 为共享合同与判定；`docs` 为服务自身文档，配置类型说明位于 docs/guides/profiles。

本机目录、remote 与最终命名核对以交付现场为准。当前源码迁移不代表发布；自动测试、独立进程恢复与真实跨 Codex 会话验收分别记录，真实会话未发生时不能宣称完成。
