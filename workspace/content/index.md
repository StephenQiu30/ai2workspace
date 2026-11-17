---
title: 知识库使用说明
description: 本地使用、文档编写、检查与静态部署。
---

知识文档统一放在 `workspace/content/`，页面、导航和全文搜索由 Fumadocs 自动生成。

## 项目文档协作

知识库按 **Requirement → Design → PRD → Plan** 展示完整需求链路。Research 保存贯穿各阶段的调研、术语和参考资料，Template 提供写作模板，项目规范说明目录与交付约定。

分类不编号，分类内的每份文档使用三位编号和中文名称，例如 `design/001-学习社区与通用Agent顶层设计.md`，标题显示为“001 · 学习社区与通用 Agent 顶层设计”。若该分类最大已用编号为 001，下一篇使用 `002-中文设计主题.md`；实际新增前以目录已用编号为准，不新建带编号的分类。

| 分类目录 | 内容 |
| --- | --- |
| `requirement/` | 原始需求、材料来源、确认和变更 |
| `design/` | 产品体验、工程职责、方案和取舍 |
| `prd/` | 产品范围、业务规则、功能和验收条件 |
| `plan/` | 任务拆解、依赖、执行证据和交接 |
| `research/` | 调研问题、来源、观察、术语和结论 |
| `template/` | 五份文档模板 |
| `standards/` | 通用文档约定与 Google Go / TS 工程规范 |

各分类独立递增编号；同一专题通过名称、稳定文档 ID 和相对链接串联，不要求跨分类序号相同。只创建有实际内容的文档，新增正文由 Fumadocs 自动发现，无需逐篇登记导航。

[项目文档与设计规范](./standards/001-项目文档与设计规范.md)说明编号、权威边界、稳定 ID 和 Skills 入口。PRD 方法复用 `pm-execution:create-prd`，执行计划复用 `superpowers:writing-plans`；项目 Skills 已适配分类内文档编号。

按实际需要使用[需求模板](./template/001-需求记录模板.md)、[设计模板](./template/002-设计文档模板.md)、[PRD 模板](./template/003-PRD模板.md)、[计划模板](./template/004-执行计划模板.md)和[调研模板](./template/005-调研记录模板.md)。模板不代表真实需求、评审或交付已经完成。

## 当前产品需求集

学习社区与通用 Agent 包含七个需求领域，分别具备独立需求记录、设计、PRD 与实施计划：[需求分类与确认](./requirement/001-学习社区与通用Agent需求记录.md#需求分类与独立交付) → [总体及领域设计](./design/001-学习社区与通用Agent顶层设计.md#领域设计索引) → [产品规则与验收](./prd/001-学习社区与通用Agent产品需求.md) → [阶段与领域计划](./plan/002-学习社区与通用Agent实施计划.md)。总览保留共同目标、原始来源和跨领域关系；具体需求、规则及任务由各领域文档维护。该入口保存规划资料，业务任务仍未开始。

## 本地使用

使用 Node.js 24、pnpm 12，在仓库根目录执行：

```sh
cd workspace
pnpm install --frozen-lockfile
pnpm dev
```

访问 http://localhost:3122/docs/ 。使用目录浏览文档，或通过搜索查找标题和正文。

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

文档之间使用相对 Markdown 链接，例如 `[使用说明](./index.md)`。需要调整目录标题或顺序时，按 [Fumadocs 页面约定](https://www.fumadocs.dev/docs/next/page-conventions)配置 `meta.json`，保留 `"..."` 自动纳入新增文件。

不要将凭据、私有配置或运行日志放入 `content/`。

## 检查与预览

在 `workspace/` 中执行：

```sh
pnpm lint
pnpm types:check
pnpm build
pnpm start
```

访问 http://localhost:3122/docs/ ，检查文档链接、中文搜索和移动端目录。开发与静态预览共用 3122，启动前先停止本项目占用该端口的实例，不停止其他项目进程。

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
