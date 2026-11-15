# Ai2sd

Ai2sd 的研发工作区。参考 Glowloom 的 Workspace 和配置规范初始化，当前包含知识库、协作规范、文档模板及工作流。

## 启动

需要 Node.js 24 与 pnpm 12.4.2：

```bash
pnpm --dir workspace install --frozen-lockfile
pnpm dev
```

打开 http://127.0.0.1:1140/。搜索与阅读无需额外服务或凭据。

## 项目入口

- [协作规范](AGENTS.md)：范围、文档、工程与 Git 规则。
- [工程配置](PROJECT.md)：技术栈、命令、端口及检查。
- [视觉基线](DESIGN.md)：组件与主题约定。
- [任务状态](BACKLOG.md)：当前进度、阻塞及下一步。
- [工作区说明](workspace/README.md)：阅读、编写、运行和验证。
- [知识库索引](workspace/content/00-文档索引.md)：原始需求、设计、PRD、计划与模板。

业务方向和业务工程尚未确定，后续从原始需求及确认开始。
