# 项目 Workspace 目录与 Harness 对齐

用户最新确认：服务作为项目内 Workspace 模块存在，不先做通用服务。当前任务 ID 为 repository-alignment，实时阶段、条件与证据只查询 CLI TaskState，不在本文重复维护。

## 本轮改动

- 正文迁至 src/content，模板迁至 harness/templates，工具迁至 scripts；保留文档 ID 和封存历史。
- 新增 code 工程关联骨架、Harness 策略、角色与交付 Skill，目录职责保持唯一来源。
- Harness 在任务创建、准备、验证时检查必需检查、角色和精确输出；策略变化使旧依据过期；基于 TaskOrigin 检查实际范围。
- 网页默认服务当前项目，通用空间创建不作为本阶段产品流程。
- 完整设计及 AC-01—AC-22 存为本项目计划，尚未发生的真实能力和验收不自报完成。

## 验证记录

验证日期：2026-11-16。目录一致性和反例检查、核心策略的准入／范围／漂移／原始现场保留测试、CLI 默认路径迁移与自定义配置保留测试已实际执行通过。类型、lint 和生产构建也已执行；正式交付候选的完整检查结果统一查询 CLI 任务与封存运行，不在正文复制第二份实时状态。

当前任务必须实际执行 repository-alignment、service-types、service-tests，以及前端／体验角色要求的 service-lint、service-format、service-build、service-browser、service-bundle。执行计划、模板和源代码不能替代这些运行。

查询：pnpm cli --workspace spaces/self task status --task repository-alignment --json。最终说明和限制保存在真实交接检查点；若任一检查失败、缺失或过期，任务不能完成。

## 独立代码评审

已按 requesting-code-review 技能进行只读 AI 评审并复核修复：补上移除工程绕过原始范围审计的缺口，固定已准备任务的工程集合，禁止重建时丢弃策略，并补充黑盒回归。POM 已收窄为交付管理和指南职责，不再单独拥有产品、架构或设计正文。复核未发现新的重要问题；AI 代码评审不代替独立人员业务验收。

## 能力边界

当前工作不声称完成 Codex 调度或 OS 级写入控制；人工验收与例外写入、其他语言工程工具及真实新 Codex 会话接续仍需各自实现和实际证据。历史自用任务、origin、快照和检查点不被重新完成或覆盖。没有提交或推送授权。
