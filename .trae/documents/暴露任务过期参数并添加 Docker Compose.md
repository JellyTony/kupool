## 目标
- 在服务端暴露任务过期参数，启用可选的 "Task expired" 校验。
- 添加标准 docker-compose，用于启动 RabbitMQ 与 PostgreSQL 测试环境。

## 代码改动
- cmd/kupool-server/main.go：
  - 新增 flag `-expire`（duration），并支持环境变量 `KUP_EXPIRE`（同样使用 time.ParseDuration）。
  - 调用 `server.NewAppServer(addr, store, queue, interval, expire)` 传入过期时间。
- app/server/server.go：
  - 将 `NewAppServer` 增加 `expire time.Duration` 参数，并传给 `NewCoordinator(..., expire)`