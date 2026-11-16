# 空间配置样例

此目录为可复制样例，不自动执行检查。workspace.yaml 是项目内 Workspace 模块骨架；development.example.yaml 展示工程与检查。当前项目使用 spaces/self/workspace.yaml，模板不是另一份运行时配置。Core 只接受内建 profile 值 generic/development，src/content/guides/profiles 中的说明不作为运行时配置加载。

1. 复制 workspace.yaml 到自行选择的可写 workspaceDir；使用唯一空间 ID，来源可保持只读。
2. 将 workspace.local.example.yaml 复制为 workspace.local.yaml，替换本机绝对路径；dataDir 默认可设 .local，两个文件后者与运行目录排除 Git。development 还需增加 service-root binding。
3. 在仓库构建 CLI：`pnpm build:cli`；执行 `pnpm cli --workspace <workspaceDir> space validate --json` 和 `source scan --json`，读取实际问题与 source/document ID。
4. 根据任务复制 ../tasks/task.example.json；按真实空间、来源、工程及 catalog 文档 ID 填写合同。scope.outputs 声明精确产出，inputs 的 required/requireConfirmed 依据实际授权，不把扫描候选自动提升为确认依据。
5. 按[本机使用与交接](../../../src/content/guides/02-本机使用与交接.md)准备、检查与交接；Codex 从仓库 .agents/skills/ai2workspace/SKILL.md 加载实际 Skill。../tasks/notes.example.json 的 contractRevision、snapshotId 和 evidenceRunIds 必须替换成该任务真实值（全零 snapshotId 仅为结构占位）；样例文本不能当证据导入。

字段权威为 [Core 合同](../../../packages/core/src/contracts.ts)与运行时 schema。检查 argv/cwd 固定在配置；artifacts 为本次运行 artifacts 目录内相对路径，程序通过 AI2WORKSPACE_RUN_DIR 定位运行根目录。现有类型/测试示例只使用 CLI 直接采集输出，因此 artifacts 为空数组。声明必需产物时须由本次进程新生成，不能引用旧工程报告。

首期 privateRefs 必须为空；默认仅 node/pnpm 版本探测和公开 NODE_ENV/CI。工具、公开环境、候选或依据变化后复核证据，不填入凭据。模板和 CLI 不授权 Git、生产数据或付费操作。
