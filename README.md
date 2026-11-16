# ai2workspace · 项目 Workspace 模块

为当前项目提供统一的文档、角色、工程关联、规范、检查证据和交接。先把本项目交付流程做实，不先建设通用知识服务或多空间管理平台。

## 开始

需要 Node.js 24、pnpm 12.4.2，从仓库根执行：

    pnpm install --frozen-lockfile
    pnpm setup:local
    pnpm dev

打开 http://127.0.0.1:1140/ 阅读当前项目。任务与交接在 /spaces/self，配置在 /spaces/self/settings。已有旧默认资料绑定时，构建后运行 pnpm setup:migrate-layout；自定义绑定和历史运行数据保持不变。

## 职责入口

- [Agent 入口](./AGENTS.md)、[工程规范](./PROJECT.md)、[视觉规范](./DESIGN.md)。
- [项目知识](./src/content/00-文档索引.md)、[模块范围](./src/content/product/requirements/03-Workspace模块范围.md)、[完整交付设计](./src/content/architecture/05-Workspace交付设计.md)。
- [代码工程关联](./code/README.md)、[Harness](./harness/README.md)、[角色职责](./harness/roles.json)、[技术基线](./harness/standards.md)。
- [真实 CLI 操作](./.agents/skills/ai2workspace/SKILL.md)、[本轮实施与验证](./src/content/development/04-目录与Harness对齐.md)。

项目正文只有 src/content 一份，模板位于 harness/templates，运行状态以实际 TaskState 为准。pnpm check:workspace 会检查目录、角色、规则、计划与实际检查入口的一致性；Harness 进一步核对任务范围、角色和证据。

设计目标、实际实现和真实验收分别记录。尚未发生的 Codex 调度、人工验收、跨会话接续或发布不得标为完成。
