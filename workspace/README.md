# ai2workspace 知识库

[文档导航](./content/00-文档导航.md) · [项目总体设计](./ARCHITECTURE.md) · [文档与工程规范](./content/01-项目文档与设计规范.md)

项目级架构在本目录的 `ARCHITECTURE.md` 维护，专题正文位于 `content/`。Fumadocs 直接读取这两处来源并生成页面、侧栏和搜索。

## 本地使用

使用 Node.js 24、pnpm 12，在仓库根目录执行：

```sh
cd workspace
pnpm install --frozen-lockfile
pnpm dev
```

访问 [本地知识库](http://localhost:3122/docs/)。使用目录浏览文档，或通过搜索查找标题和正文。

## 编写文档

在 `workspace/content/` 下新增 `.md` 文件，使用 frontmatter 声明标题和简介，正文从二级标题开始：

```md
---
title: 文档标题
description: 简短介绍
---

## 内容

文档正文。
```

新文档自动进入页面、目录和搜索，无需修改路由或维护文件清单。需要 React 组件时使用 `.mdx`。

文档之间使用相对 Markdown 链接。目录标题与首页顺序沿用 [Fumadocs 页面约定](https://www.fumadocs.dev/docs/next/page-conventions)。

`.local/` 是忽略提交的本机资料目录。凭据、私有配置和运行日志留在知识库内容源之外。

## 检查与预览

在 `workspace/` 中执行：

```sh
pnpm lint
pnpm types:check
pnpm build
pnpm start
```

打开[本地知识库](http://localhost:3122/docs/)，检查文档链接、中文搜索和移动端目录。开发与静态预览共用 3122，切换前先停止正在运行的本项目实例。

## 部署

`workspace/` 可独立构建。静态托管设置如下：

| 设置 | 值 |
| --- | --- |
| 工作目录 | `workspace`，单独部署模块时为模块根目录 |
| 安装命令 | `pnpm install --frozen-lockfile` |
| 构建命令 | `pnpm build` |
| 发布目录 | `out` |

将 `workspace/out/` 整体发布到支持目录索引的静态托管服务。页面 URL 使用末尾斜杠，托管服务应提供对应目录的 `index.html`，不存在的页面返回 `404.html`。

新增或修改文档后重新构建并发布。发布产物不需要数据库、后端常驻进程或私有环境变量。
