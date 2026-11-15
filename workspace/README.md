# Ai2sd Workspace

采用 Glowloom 当前 Workspace 的 Next.js、Fumadocs MDX、官方 Fumadocs UI 和 shadcn Base Nova / Neutral 配置。独立的 Ai2sd 内容、文档 ID 与本机端口；搜索采用官方内置方案。

## 运行

```bash
pnpm install --frozen-lockfile
pnpm dev
# 正式构建与本机预览
pnpm build
pnpm start
```

入口：http://127.0.0.1:1140/，首页跳转 /read。无需环境变量、密钥或其他服务。dev 与 start 使用同一端口，按需运行其中一个。

## 内容

主链为 requirement → design → prd → plan；research、operations、templates、skills、workflows 分别承载补充职责。实施状态只在根 BACKLOG.md 维护。

1. 在分类内创建唯一的 NN-名称.md，正文 H1 不重复编号，相对链接使用 ./ 或 ../。
2. 在 content/document-ids.json 添加唯一、稳定的 id 与 path，页面地址为 /read/ID；移动正文保留 ID。
3. 更新分类 00 导航和 meta.json，必要时更新根知识库索引。
4. 执行检查并在浏览器核对导航、链接、搜索、Markdown 和移动端。

支持代码高亮、Mermaid 和官方 Markdown 复制。/read.md、/read/ID.md、/llms.txt、/llms-full.txt 提供只读文本。根规范保留原文件，知识站只发布 content 中的正文；源码与本机路径不自动发布。

## 验证

```bash
pnpm lint
pnpm types:check
pnpm format:check
pnpm build
pnpm test:e2e
```

Playwright 默认启动本项目正式构建，需先 pnpm build；首次准备浏览器可执行 `pnpm exec playwright install chromium`。也可通过 WORKSPACE_TEST_URL 指定已有预览地址。测试只读站点，不连接业务或收费服务。

## 部署

采用标准 Next.js build/start；部署平台的项目根目录设为 workspace。监听主机与端口可由 Next.js CLI 参数覆盖。站点是免登录知识库，上线前由部署方确定访问范围；本次初始化仅做本机预览，不创建远端资源。
