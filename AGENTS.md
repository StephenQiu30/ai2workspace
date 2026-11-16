@/Users/stephenqiu/.codex/RTK.md

# 项目协作入口

使用中文，先说明结果，再给必要证据与限制。开始前核对项目目录、Git 根目录、分支与工作区，保留用户和并行任务已有修改。

- 当前产品是知识库文档网站；网站工程只放在 `workspace/`，正文只放在 `workspace/content/`，不得复制两份。
- 基于 Fumadocs 官方脚手架和组件，自动发现 Markdown/MDX、生成导航及搜索；不添加逐页路由、硬编码文档清单、自定义解析器或旧版 Core/CLI/空间管理封装。
- 使用、编写与部署见 [知识库使用说明](./workspace/content/index.md)，Fumadocs 入口见 [workspace/AGENTS.md](./workspace/AGENTS.md)。界面使用官方 Spacious 布局及配套样式，主题变量与字体对齐官网；TypeScript 遵循 Google TypeScript 规范并保持 strict。
- 知识编写保留必要来源，区分需求、建议与实际证据。新增知识通过 frontmatter 声明标题，目录按需用 Fumadocs meta.json 组织。
- 项目文档遵循[文档与设计规范](./workspace/content/standards/project-design.md)，四类模板统一位于 `workspace/content/templates/`。按任务选择并读取[需求 Skill](./.agents/skills/workspace-requirements/SKILL.md)、[设计 Skill](./.agents/skills/workspace-design/SKILL.md)、[PRD Skill](./.agents/skills/workspace-prd/SKILL.md)或[计划 Skill](./.agents/skills/workspace-plan/SKILL.md)，直接引用模板与规范，不复制规则或引入任务运行系统。
- 不读取、复制或提交无关凭据、私有环境文件、运行数据及生成物；`workspace/.local/` 仅保存忽略提交的本机资料，不得接入公开文档源。
- 未经明确要求，不提交、推送、建远程分支或 PR、合并、打标签或发布。获授权提交时仅含任务范围，标题用 `type(scope): 中文说明`。
- 完成前核对改动，执行 lint、类型、生产构建和适用的交互浏览器验收；未执行、失败及阻塞如实记录。
