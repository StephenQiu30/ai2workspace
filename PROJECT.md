# ai2workspace 项目 Workspace 模块

## 定位与布局

当前服务是项目内 Workspace 模块，先服务本项目的文档、角色、工程关联和交付。不先建设通用知识治理、多空间平台或通用流程引擎。产品范围见 [模块需求](./src/content/product/requirements/03-Workspace模块范围.md)，目录唯一约定见 [职责与归属](./src/content/architecture/03-职责与文档归属.md)。

- src/content：项目知识正文；harness/templates：可复制模板；根规范保持唯一来源。
- code：工程关联说明与共享骨架；运行身份与检查以 spaces/self/workspace.yaml 为准。
- harness：角色、策略、规范入口、任务计划；运行状态以 Core 的 TaskState 为准。
- scripts：一致性检查、初始化和默认绑定迁移。
- apps/web、apps/cli、packages/core：Workspace 模块自身界面、CLI 和共享判定；Core 不依赖 React/Next.js。

Node.js 24、pnpm 12.4.2；精确依赖以 package.json 和锁文件为准。默认本机入口 http://127.0.0.1:1140/ 直接阅读当前项目，任务与配置入口分别为 /spaces/self 和 /spaces/self/settings。历史空间路由保留兼容，不提供新通用空间创建流程。

## 初始化与检查

从仓库根执行：

    pnpm install --frozen-lockfile
    pnpm setup:local
    pnpm dev

已有旧默认绑定时，先构建 CLI，再显式运行 pnpm setup:migrate-layout。该操作只迁移已知旧默认路径，不覆盖自定义绑定和历史任务。dev 与 start 择一占用端口，不停止其他项目进程。

适用交付检查：pnpm check:workspace、pnpm lint、pnpm types:check、pnpm format:check、pnpm test、pnpm build、pnpm test:e2e。Playwright 使用正式构建；用 WORKSPACE_TEST_PORT 指定隔离端口。验证范围和结果见实际运行证据，不能以命令已定义替代执行。

## Harness 准入与交付

先读取 [Harness 约定](./harness/README.md)和 [项目 Skill](./.agents/skills/ai2workspace/SKILL.md)。任务显式选择 roleIds 和精确输出，必需检查来自 harness/policy.json；角色职责来自 harness/roles.json。策略或角色变更须显式重建快照，实际范围自 TaskOrigin 核对，不能通过重建洗掉越界。输出检查是产物接收机制，不是 Codex 写入沙箱。

knowledge-root 绑定 src/content，templates-root 绑定 harness/templates，service-root 绑定本工程。共享声明不保存绝对路径；workspace.local.yaml 和运行 dataDir 私有。必需检查使用配置内固定 argv，运行结果绑定候选、工具、公开环境和产物。

当前模块支持资料读取、显式上下文、声明检查与交接。Codex 调度、人工验收或例外写入、私有环境证明、外部报告核验未实现时如实阻塞，不宣称完整设计已通过。真实跨会话验收必须实际发生。

## 工程规范

前端遵守 Vercel Web Interface Guidelines，TypeScript 遵守 Google TypeScript Style Guide 并保持 strict；React/Next.js 采用 Vercel React Best Practices。Go、Java、Python 工程实际纳入时分别采用 Google Go、Alibaba Java、Google Python 规范，未绑定的语言不作为通用平台建设任务。来源与验证边界统一见 [技术规范](./harness/standards.md)。

如引入数据库，完整 schema.sql 为结构唯一来源，data.sql 为公共种子；不增加独立增量 migrations 或运行时动态 DDL。新空库初始化与已有库受控变更分别处理。

不自动提交、推送、发布、重置或清理已有修改。权限来自用户授权，CLI、Skill 和文档不会增加授权。
