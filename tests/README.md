# ai2workspace 跨模块测试

测试与业务源码分离。这里使用固定版本的 Playwright，通过真实 HTTP 入口验证接口契约、SSR 初始连接、浏览器刷新，以及浏览器请求中断后的恢复；不导入 Go 私有包。

准备 Chrome，并先启动前后端或容器组合，然后在本目录执行：

```sh
pnpm install --frozen-lockfile
pnpm test
```

桌面和移动端分别运行相同验收。错误恢复场景通过浏览器路由中断模拟网络故障，其余接口与服务端渲染连接真实 Go 服务。测试报告位于忽略提交的 `test-results/`，不进入业务镜像。
