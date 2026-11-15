# ai2workspace Web

本机多空间知识阅读、空间声明编辑和治理结果展示。采用官方 Fumadocs UI、内容树、搜索与 Markdown/LLM 接口，沿用 shadcn Base Nova / Neutral 主题。普通 Markdown 在服务端安全渲染；正文中的 JavaScript、MDX 表达式和原始 HTML 不会执行。

## 运行

从仓库根目录安装依赖和构建 Core，再运行 Web：

```bash
pnpm install --frozen-lockfile
pnpm --filter @ai2workspace/core build
pnpm --filter @ai2workspace/web dev
# 正式构建与本机预览
pnpm --filter @ai2workspace/web build
pnpm --filter @ai2workspace/web start
```

默认入口为 http://127.0.0.1:1140/；`/read/ID` 保留自用空间的稳定地址，`/spaces` 选择空间。Web 只扫描已登记的知识来源，不执行声明中的工程检查。任务创建、检查、交接和恢复使用 CLI。

## 空间与知识

空间登记目录由服务启动环境 `AI2WORKSPACE_SPACES_DIR` 指定，默认仓库 `spaces/`。仅枚举其中包含 `workspace.yaml` 的普通子目录；网页不能输入任意本机路径。每个空间目录维护共享声明、私有 `workspace.local.yaml` 绑定及 Core 目录身份。

网页可创建和编辑共享 YAML 声明，保存前使用 Core 合同校验。已有空间 ID 不可变，保存受空间执行锁和配置修订检查保护。新空间生成空的私有绑定文件，使用者在本机配置目录路径。网页不展示私有绑定或原始进程输出。并发创建由登记目录的 `.registry-create.lock` 保护；异常退出遗留锁保留 pid/host，须在本机确认持有进程已结束后显式处理，不因时间自动清理。

原始资料保留在绑定目录。Core 扫描并保存不可变知识修订；页面、目录、相对链接、搜索、Markdown 与 LLM 输出使用同一修订。旧修订地址保留；来源失联时展示上次完整索引并标明陈旧，恢复后可点击“读取最新知识”。文本接口通过 `X-Knowledge-Revision` 和 `X-Knowledge-Stale` 声明修订与陈旧状态。

多空间阅读地址为 `/spaces/空间/read/来源/文档ID`。自用空间另保留 `/read`、`/read/ID`、`/read.md` 和 `/read/ID.md`；`/llms.txt` 与 `/llms-full.txt` 支持 `space`、`revision` 查询参数。标准 Markdown 支持 GFM 表格、官方代码高亮和按需加载 Mermaid。绑定内 PNG、JPEG、GIF、WebP、AVIF 附件由 Core 校验边界后读取；排除目录、私有文件和符号链接不能通过图片路由访问。

## 验证

从仓库根目录执行：

```bash
pnpm --filter @ai2workspace/web test
pnpm --filter @ai2workspace/web lint
pnpm --filter @ai2workspace/web types:check
pnpm --filter @ai2workspace/web format:check
pnpm --filter @ai2workspace/web build
pnpm --filter @ai2workspace/web test:bundle
pnpm --filter @ai2workspace/web test:e2e
```

Playwright 默认启动正式构建，测试空间配置与运行数据位于临时目录。自用正文只读绑定，第二空间包含隔离测试资料；配置编辑测试只写临时声明。可用 `WORKSPACE_TEST_PORT=1141` 避免占用默认服务；`WORKSPACE_TEST_URL` 指向已有服务时跳过配置/资料修改测试。测试覆盖搜索冷加载首击、多空间隔离、修订一致、来源失联、普通 Markdown 不执行表达式、安全图片、声明保存和冲突、移动端和主题切换。

## 部署边界

采用本机 Node.js 的 Next.js build/start，不支持仅静态托管。运行时空间目录和私有数据不进入 Web 构建包，必须由本机提供。服务绑定 127.0.0.1；本机验证不代表远端部署或多用户服务验收。
