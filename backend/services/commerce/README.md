# commerce 微服务

ai2workspace 交易退款与权益服务 (Commerce Service)。

- 端口：`:3125`
- 职责：独占课程价格、订单生命周期、支付尝试与幂等确认、购买权益授予（entitlements）以及 7×24h 人工退款与权益撤销闭环。
- 架构约束：严禁手写 Swagger 文件或前端契约；通过 Swaggo 单向注解生成。无过度描述的 `/v1` 路由前缀。
