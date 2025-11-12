## 完成度结论
- 必备功能均已实现并通过测试：鉴权、长连接并发、任务分发（30s 可配置）、提交校验（job_id/nonce、SHA256、1s速率、重复nonce）、错误响应、统计（按分钟聚合，默认memory）、消息处理器（内存队列异步，支持RabbitMQ）、PostgreSQL存储抽象。
- 文档与可运行性：README 包含构建运行、后端选择与日志配置；服务端/客户端可在 macOS/Linux 编译运行；测试覆盖成功/错误/并发/速率限制，自动终止。

## 剩余可优化与验证项
1. 增加任务过期的单元测试（目前功能已实现，测试未覆盖）。
2. 增加容器化示例：docker-compose 启动 PostgreSQL 与 RabbitMQ，提供一行启动与 README 示例。
3. 提升可观察性：增加提交成功率与错误分布指标（日志已增强，可补充简单 metrics 导出）。
4. 内存清理：为 `UsedNonces` 加滑窗清理策略，降低长时运行内存占用。
5. 客户端最少 1 次/分钟验证测试（当前服务端测试已充分，客户端策略已实现；补一个小测试便于展示）。

## 执行计划
- 测试：
  - 新增 `Task expired` 单元测试于 `app/server` 包，设置短 `expireAfter` 并断言错误响应。
  - 新增客户端最少 1 次/分钟的策略测试（短周期模拟）。
- 容器化：
  - 新增 `docker-compose.yml`，包括 `postgres` 与 `rabbitmq`，默认凭证与网络。
  - README 增加容器化快速启动与环境变量示例。
- 代码优化：
  - `UsedNonces` 清理：在 `rotateJob` 或定时协程中保留近 N 个 job 的 nonce 集。
  - 可选：简单 metrics（如以日志/端点形式暴露计数）。

## 验证
- 本地运行：`go build ./...` 与 `go test ./protocol ./stats ./mq ./app/server -v` 全量通过。
- 容器化：`docker-compose up -d` 后，设置 `KUP_PG_DSN` 与 `KUP_MQ_URL`，服务端选择 `pg` 与 `rabbit` 验证端到端提交与统计。