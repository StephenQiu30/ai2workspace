# Workspace 知识库

基于 Fumadocs 官方脚手架的独立静态文档网站，正文唯一位置为 `content/`。页面、导航和搜索从内容自动生成。

使用 Node.js 24、pnpm 12，在本目录执行：

```sh
pnpm install --frozen-lockfile
pnpm dev
```

默认访问 http://localhost:3000/docs/ 。执行 `pnpm build` 生成 `out/`，`pnpm start -l 1140` 预览静态产物。

完整的使用、编写、检查与部署方法见 [知识库使用说明](./content/index.md)。

`.local/` 是忽略提交的本机资料，不是文档源，不参与静态构建或发布。
