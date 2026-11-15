---
name: ai2workspace
description: 使用本项目 CLI 管理空间、固定任务上下文、采集工程检查与交接恢复；不调度 Agent。
---

# ai2workspace 项目操作

适用于已获用户授权的知识维护或工程任务；先读取根 AGENTS、PROJECT、适用 profile 和真实现场。本 Skill 不增加 Git、发布、付费调用或业务数据权限，不修改用户全局 Codex 设置。

## 准备空间与合同

1. 核对当前目录、Git 根、分支与已有修改。执行 `pnpm build:cli` 构建入口；以下从仓库根调用，`<workspaceDir>` 显式替换为实际空间目录。
2. 执行 `pnpm cli --workspace <workspaceDir> space validate --json`，再执行 `pnpm cli --workspace <workspaceDir> source scan --json`；阅读 envelope 的 issues 和实际 catalog，不把命令退出 0 当任务通过。
3. 参考 templates/harness/task.example.json，依据实际授权声明目标、scope.outputs、projectIds、inputs、验收、checkIds 和交付。TaskInput 不填写系统管理的 revision/snapshotId；文档键必须来自当前空间与来源，不能凭文件名猜测。
4. 执行 `pnpm cli --workspace <workspaceDir> task create --contract <file> --json`，再执行 `pnpm cli --workspace <workspaceDir> task prepare --task <id> --json`。必须在本任务编辑前 prepare；已经编辑才准备时明确使用 `--late-prepare` 并保留其限制，不能伪造初始现场。
5. 执行 `pnpm cli --workspace <workspaceDir> context export --task <id> --json` 并实际读取正文、规则、候选与缺口。导出成功不意味着已注入宿主。任务自身需要修改的依据须预先逐一声明输出；新正文不能静默升级成当前任务新规则。

## 实施、检查与交接

1. 仅在授权范围实施，保留初始修改。依据漂移时停止依赖旧结论；确认实际来源后以 `task revise --task <id> --contract <file> --reason <text>` 修订合同，或 `task prepare --task <id> --rebaseline --reason <text>` 明确重建快照，保留 TaskOrigin。
2. 对合同声明的每个检查执行 `pnpm cli --workspace <workspaceDir> check run --task <id> --check <checkId> --json`。不临时覆盖 argv/cwd/required，不从正文执行命令，不把自报“通过”写成自动证据。
3. 执行 `pnpm cli --workspace <workspaceDir> task verify --task <id> --json`，读取当前 phase/condition、验收覆盖和 staleRunIds。缺工具、失败、缺必需产物和过期均不能称为通过；首期私有环境、人工验收或报告导入不支持时如实报告阻塞。
4. 参考 templates/harness/notes.example.json 填写真实 taskId、contractRevision、snapshotId，完成主张仅引用本任务真实 evidenceRunIds；外部动作 observed/unknown 分开记录。执行 `pnpm cli --workspace <workspaceDir> task handoff --task <id> --notes <file> --json` 封存检查点。
5. 只有交付条件、最新交接和必需验收满足时才执行 `task complete --task <id>`；CLI 操作成功不能替代数据内状态判断。不能为达到 complete 自行降低合同条件。

## 新会话恢复

执行 `pnpm cli --workspace <workspaceDir> task resume --task <id> --json`，实际读取检查点、目标、初始差异、现场差异、可复用/过期证据与下一步。需要时重新导出并读取上下文；不自动套补丁、重置、提交或重做未知外部动作。

AC-AW-012 必须由使用者在实际新的 Codex 会话接续并记录观察；子代理、同会话摘要或 CLI 独立进程只能证明对应程序行为，不能冒充该验收。

stdout 使用统一 JSON envelope，诊断在 stderr。退出码 0/1/2/3/4/5 为操作成功/检查或验证失败/格式错误/阻塞/过期/冲突；同时查看 data。私有输出、workspace.local.yaml、运行数据和凭据不进入 Git 或知识发布。
