@/Users/stephenqiu/.codex/RTK.md

# 项目协作入口

使用中文，先说明结果，再给必要证据与限制。开始前核对项目目录、Git 根目录、分支与工作区，保留用户和并行任务已有修改。

项目与产品标识统一为 `ai2workspace`。测试代码采用独立 `tests/` 目录，不与业务源码混放；具体归属及 Go 可见性边界见工程规范和 `PROJECT.md`。

工具接入直接使用官方 CLI 与配置文件，不新增 `scripts/` 目录或独立编排脚本。

- 当前已实现知识库文档网站；知识库网站工程只放在 `workspace/`，知识正文只放在 `workspace/content/`，不得复制两份。业务工程分别位于 `frontend/`（Next.js）与 `backend/`（Go 微服务），完整选型与实施前提见根目录 [PROJECT.md](./PROJECT.md)，已初始化技术基线，业务功能仍待需求定义。
- 业务前端采用 Next.js + TypeScript + React + Tailwind CSS + shadcn/ui（Radix 版本）+ Prettier + ESLint + Axios + Umi OpenAPI。业务接口从 Go DTO 与 Swaggo 注释生成 Swagger，再生成前端类型和调用代码；禁止手动维护 Swagger / OpenAPI 文件及生成的调用代码。各 Go 服务独立维护依赖、数据所有权和部署边界。
- 基于 Fumadocs 官方脚手架和组件，自动发现 Markdown/MDX、生成导航及搜索；不添加逐页路由、硬编码文档清单、自定义解析器或旧版 Core/CLI/空间管理封装。
- Go 服务遵循 Google Go，前端 TypeScript 遵循 Google TS 并保持 strict；命名、目录职责、依赖方向及框架 / 生成代码例外统一见[工程规范](./workspace/content/standards/001-项目文档与设计规范.md#工程规范与目录设计)，不在其他文件复制规则正文。
- 使用、编写与部署见 [知识库使用说明](./workspace/content/index.md)，Fumadocs 入口见 [workspace/AGENTS.md](./workspace/AGENTS.md)。知识库界面使用官方 Spacious 布局及配套样式，主题变量与字体对齐官网。
- 知识编写保留必要来源，区分需求、建议与实际证据。新增知识通过 frontmatter 声明标题，目录按需用 Fumadocs meta.json 组织。
- 项目文档遵循[文档与设计规范](./workspace/content/standards/001-项目文档与设计规范.md)，主链路按 requirement → design → prd → plan 排列，分类目录不编号，分类内文档采用 `NNN-中文文档名称.md` 及对应的三位标题编号，调研资料位于 `workspace/content/research/`，五类模板统一位于 `workspace/content/template/`。按任务选择并读取[需求 Skill](./.agents/skills/workspace-requirements/SKILL.md)、[设计 Skill](./.agents/skills/workspace-design/SKILL.md)、[PRD Skill](./.agents/skills/workspace-prd/SKILL.md)或[计划 Skill](./.agents/skills/workspace-plan/SKILL.md)，直接引用模板与规范，不复制规则或引入任务运行系统。
- PRD 编写与评审通过项目入口读取并采用 `pm-execution:create-prd`；多步实施计划采用 `superpowers:writing-plans`。项目 Skill 仅适配知识目录、资料关联和交付记录，使用时核对宿主实际可用能力，不把插件方法复制进仓库或将计划条目视作执行授权。
- 不读取、复制或提交无关凭据、私有环境文件、运行数据及生成物；`workspace/.local/` 仅保存忽略提交的本机资料，不得接入公开文档源。
- 未经明确要求，不提交、推送、建远程分支或 PR、合并、打标签或发布。获授权提交时仅含任务范围，标题用 `type(scope): 中文说明`。
- 完成前核对改动，执行 lint、类型、生产构建和适用的交互浏览器验收；未执行、失败及阻塞如实记录。
