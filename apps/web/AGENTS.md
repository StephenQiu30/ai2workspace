# Web 协作规则

继承根 AGENTS.md。这里维护当前项目 Workspace 模块的阅读、配置和交接展示，不先建设通用平台。

- 使用官方 Fumadocs UI、内容树、搜索和 Markdown/LLM 能力；中性主题沿用根 DESIGN.md。
- 知识与任务事实来自 `@ai2workspace/core`，不得在 Web 重复实现身份、治理、检查或交接状态机。
- 只从 `AI2WORKSPACE_SPACES_DIR` 登记的普通子目录选择空间。本机路径绑定及运行数据不发送到客户端，不打包进构建。
- Markdown 在服务端安全处理；不启用原始 HTML、MDX JavaScript 执行或正文命令执行。附件调用 Core 的边界校验接口。
- HTML、目录、搜索、Markdown 和 LLM 文本必须采用同一知识修订；旧内容和失联来源明确标记。
- 网页仅编辑当前项目共享声明并展示治理、任务、证据和交接；不发起编码、工程检查或运行进程。
- 保存配置使用 Core 校验、空间锁、修订冲突检测和原子写入；保留用户与并行修改。
- 修改后执行 README.md 中适用的 lint、类型、格式、构建和浏览器检查。

官方文档：https://fumadocs.dev/docs/next；API 以安装版本为准。
