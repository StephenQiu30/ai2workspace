# 工程规范基线

本文件是适用规范入口；角色归属以 [roles.json](./roles.json) 为准，必需检查以 [policy.json](./policy.json) 为准。规则来源、检查能力和人工验收不能混为一谈。

| 对象            | 权威规范                                                                          | 验证方式                                                 |
| --------------- | --------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Web 界面        | [Vercel Web Interface Guidelines](https://vercel.com/design/guidelines)           | 静态检查、键盘和焦点、表单反馈、页面状态及浏览器交互验收 |
| React / Next.js | [Vercel React Best Practices](https://github.com/vercel-labs/agent-skills)        | 仅适用采用相应框架的工程，结合构建、性能与专业复核       |
| TypeScript      | [Google TypeScript Style Guide](https://google.github.io/styleguide/tsguide.html) | strict 类型检查、ESLint、格式检查及命名和类型设计复核    |
| Go              | [Google Go Style Guide](https://google.github.io/styleguide/go/)                  | gofmt 输出检查、go vet、编译测试及技术复核               |
| Java            | [Alibaba Java 开发手册](https://github.com/alibaba/p3c)                           | 采用已验证兼容的 Alibaba 检查规则、编译测试及技术复核    |
| Python          | [Google Python Style Guide](https://google.github.io/styleguide/pyguide.html)     | 固定 Pylint 与 formatter 配置、测试及专业复核            |

本仓库实际语言为 TypeScript；尚未接入的 Go、Java、Python 工程不得显示为已经验证支持。具体工具版本由锁文件及任务快照固定，外部规范采用版本和完整覆盖映射仍按完整设计的 P0 验证。规则升级有记录地采用，不自动改变进行中任务依据。

代码格式检查不能证明业务、体验或架构全部符合规范。检查失败和缺失保持真实结果；人工确认、例外写入、真实 Codex 执行适配和跨会话验证未落地时，明确记录为计划或阻塞，不用说明文档冒充服务能力。
