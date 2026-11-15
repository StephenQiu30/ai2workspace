# Workspace 协作规则

继承根 AGENTS.md。这里维护 Ai2sd 的 Fumadocs 文档站。

- 使用官方 Fumadocs UI、内容源、搜索和 Markdown/LLM 路由；主题沿用根 DESIGN.md。
- 正文位于 content/，标准 Markdown；每类文件 NN 编号唯一，00 为导航；稳定 ID 在 content/document-ids.json 维护。
- 相对链接显式使用 ./ 或 ../；重命名同步导航与映射，已有文档 ID 不变。
- 初始搜索使用官方内置多语言实现，不要求外部搜索服务、凭据或数据库。
- 不复制 Glowloom 业务内容、Logo、私有环境或生成产物；不预建登录、执行桥或任务服务。
- 根规范保留原位置，不在 content 内复制为第二份权威正文。
- 修改后执行 README.md 中适用的 lint、类型、格式、构建和浏览器检查。

官方文档：https://fumadocs.dev/docs/next，实际 API 以安装版本为准。
