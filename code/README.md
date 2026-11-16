# 代码工程关联

一个业务产品可以关联多个代码工程。共享工程身份、职责、binding 和输入边界的可执行唯一来源为 spaces/self/workspace.yaml 中的 projects；其他空间使用各自 workspace.yaml。

projects.example.json 和 projects.schema.json 是可复制的登记骨架与格式约束，不是另一份运行时工程注册表。要启用工程，必须将登记纳入目标空间并验证本机绑定。

本机绝对路径、个人选择和私有环境放在不入 Git 的 projects.local.json 或 workspace.local.yaml，不能混入共享资料或被误认为服务已加载。当前服务运行绑定以 workspace.local.yaml 为准，projects.local.json 只是保留给外部工具使用的私有格式，不自动导入。

Workspace 工具本身的 apps/ 与 packages/ 留在本仓库；用户业务源码可独立存放，通过空间 binding 关联，不能为了符合示例目录而复制业务工程。
