# ai2workspace 协作入口

使用中文，先说明结果，再给必要证据与限制。开始前核对项目目录、Git 根目录、分支与工作区；保留用户和并行任务的已有修改。

- 服务职责、文档归属和模块依赖见 [职责约定](./docs/architecture/03-职责与文档归属.md)。
- 通用知识规则见 [generic profile](./docs/guides/profiles/generic.md)，本服务工程规则见 [development profile](./docs/guides/profiles/development.md)。空间的 profile 决定适用规则，普通资料无需研发文档链。
- 运行与检查见 [PROJECT.md](./PROJECT.md)，视觉基线见 [DESIGN.md](./DESIGN.md)，知识编写见 [编写规范](./docs/guides/authoring/05-知识编写规范.md)。根规范保留唯一来源，不复制正文。
- 任务合同、CLI 和证据语义见 [设计 7.5—7.9](./docs/architecture/02-知识治理与Harness设计.md#75-任务合同与上下文快照)；实际操作见 [项目 Skill](.agents/skills/ai2workspace/SKILL.md)。CLI 不授予新权限；导出上下文后须实际读取。
- CLI 已登记任务以 TaskState 为状态权威；[BACKLOG.md](./BACKLOG.md)维护尚未纳入 CLI 的迁移事项和人工验收缺口，不重复手工维护同一任务。
- 公开事实、观察、建议和待确认分开记录。保留原始需求原文、来源、稳定 ID 与确认结论；评审通过不等于实现、验收或发布完成。
- 不读取、复制或提交无关凭据、私有环境文件、运行数据及生成物；破坏性操作和真实业务数据变更须有明确授权，不擅自重置、清理或覆盖。
- 未经明确要求，不提交、推送、建远程分支或 PR、合并、打标签或发布。获授权提交时仅含任务范围，标题用 `type(scope): 中文说明`（type 为 test/docs/impl/feat/refactor/chore，scope 为明确小写英文领域）。
- 完成前核对改动、执行适用检查及交互浏览器验收，说明关键文件、结果、限制与提交状态；未执行、失败、跳过和阻塞如实记录。
