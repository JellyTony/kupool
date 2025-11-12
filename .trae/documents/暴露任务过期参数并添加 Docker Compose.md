## 目标
- 在服务端暴露任务过期参数，启用可选的 "Task expired" 校验。
- 添加标准 docker-compose，用于启动 RabbitMQ 与 PostgreSQL 测试环境。

## 代码改动
- cmd/kupool-server/main.go：
  - 新增 flag `-expire`（duration），并支持环境变量 `KUP_EXPIRE`（同样使用 time.ParseDuration）。
  - 调用 `server.NewAppServer(addr, store, queue, interval, expire)` 传入过期时间。
- app/server/server.go：
  - 将 `NewAppServer` 增加 `expire time.Duration` 参数，并传给 `NewCoordinator(..., expire)`。
  - 其余逻辑不变。

## 配置文件
- 新增 `docker-compose.yaml`：
  - rabbitmq（management）：端口 `5672`, `15672`，持久卷。
  - postgres：端口 `5432`，设置 `POSTGRES_USER/POSTGRES_PASSWORD/POSTGRES_DB`，持久卷。

## 验证
- 编译 `go build ./...`。
- 运行服务端：可通过设置 `-expire=2m` 或 `KUP_EXPIRE=2m` 验证任务过期错误。
- 使用 compose 启动依赖并通过环境变量配置 MQ 与 PG。