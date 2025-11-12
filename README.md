KuPool 基于 TCP 的消息处理系统

概述
- 目标：实现长连接的任务分发与结果提交系统，含客户端、服务端、协议与统计处理。
- 技术栈：Golang、TCP 自定义帧协议、RabbitMQ（可选）、PostgreSQL（统计存储）。
- 运行环境：macOS 或 Linux。

架构
- 服务器：`cmd/kupool-server/main.go` 启动，装配 `AppServer`（`app/server/server.go:17`）。
- 客户端：`cmd/kupool-client/main.go` 启动，事件驱动 `Run` 循环（`app/client/client.go:34`）。
- 协调器（任务分发）：每 30 秒生成新 `server_nonce`、递增 `job_id` 并广播（`app/server/coordinator.go:39–61, 70–89`）。
- 监听器（结果处理）：校验提交、限速、去重与错误响应（`app/server/listener.go:49–88, 90–95`）。
- 统计存储：服务端消费 MQ 事件并按分钟聚合写入 Postgres（`app/server/server.go:29–37`, `stats/pg.go:30–39`）。

协议
- 请求（需要响应）使用 `Request{ id, method, params }`；广播（不需要响应）使用 `id: null`。
- 操作码：`OpBinary` 传输 JSON 请求/响应；`OpClose` 优雅断开（`server.go:115–122`）。

认证流程
- 客户端 → 服务端
  - `{"id":1,"method":"authorize","params":{"username":"admin"}}`
- 服务端 → 客户端
  - `{"id":1,"result":true}`
- 参考代码
  - 服务端接受并注册会话：`app/server/acceptor.go:22–60`
  - 客户端握手与单连接约束：`tcp/client.go:61–95`

任务分发
- 服务端 → 客户端（广播）
  - `{"id":null,"method":"job","params":{"job_id":N,"server_nonce":"..."}}`
- 服务端要求
  - 每 30 秒更新一次 `server_nonce`、递增 `job_id` 并广播。
  - 每会话仅维护最新 `server_nonce`；可选维护历史记录。
- 参考代码
  - 生成与广播：`app/server/coordinator.go:54–61, 70–89`
  - 会话状态：`app/server/types.go:9–16`

结果提交
- 客户端 → 服务端
  - `{"id":X,"method":"submit","params":{"job_id":N,"client_nonce":"...","result":"sha256(server_nonce+client_nonce)"}}`
- 服务端 → 客户端 响应
  - 成功：`{"id":X,"result":true}`
  - 失败：`{"id":X,"result":false,"error":"..."}`
- 客户端要求
  - 收到 `job` 立即计算并提交一次；随后最多 1 次/秒、最少 1 次/分钟。
  - 事件驱动循环：读协程 + `select` 处理定时器（`app/client/client.go:41–55, 104–141`）。
- 服务端要求
  - 校验 `job_id` 与 `server_nonce`；校验结果正确性；限速 1 秒最多一次；检测重复 `client_nonce`。
  - 错误条件：任务不存在、任务过期（可选）、结果错误、过频、重复提交。
- 参考代码
  - 校验与错误响应：`app/server/listener.go:49–88, 90–95`

优雅关闭
- 客户端监听 `SIGINT`/`SIGTERM` 并发送 `OpClose`：`cmd/kupool-client/main.go:25–34`。
- 服务端监听 `SIGINT`/`SIGTERM` 并调用 `Shutdown`：`cmd/kupool-server/main.go:74–77`。

统计收集
- 表结构（自动创建）：
  - `CREATE TABLE submissions(username VARCHAR(255), timestamp TIMESTAMP, submission_count INT, PRIMARY KEY(username,timestamp));`
- 聚合写入：按分钟聚合并 `UPSERT`（`stats/pg.go:30–39`）。

消息处理器（可选加分）
- RabbitMQ 发布/消费提交事件，持久化队列，支持 ACK/NACK 与重启恢复：`mq/rabbitmq.go:19–76`。
- 内存队列用于本地开发：`mq/memory.go`。

构建与运行
- 依赖：`Go 1.20+`、可选 `Docker`。
- 构建：
  - `go build ./...`
- 启动依赖（可选）：
  - `docker compose up -d`（使用项目根的 `docker-compose.yaml` 启动 RabbitMQ 与 Postgres）。
- 启动服务端：
  - `KUP_LOG_LEVEL=info KUP_MQ=rabbit KUP_MQ_URL=amqp://guest:guest@localhost:5672/ KUP_MQ_QUEUE=kupool_submissions KUP_STORE=pg KUP_PG_DSN=postgres://kupool:kupool@localhost:5432/kupool?sslmode=disable go run cmd/kupool-server/main.go -addr :8080 -interval 30s -expire 0`
  - 说明：将 `KUP_STORE=memory` 可改为内存统计；`-expire` 支持任务过期校验，`0` 为禁用。
- 启动客户端：
  - `go run cmd/kupool-client/main.go -addr localhost:8080 -username admin`

配置说明
- 环境变量（服务端）：
  - `KUP_ADDR`：监听地址（如 `:8080`）
  - `KUP_INTERVAL`：任务轮换间隔（如 `30s`）
  - `KUP_EXPIRE`：任务过期时长（如 `2m`，`0` 为禁用）
  - `KUP_STORE`：`memory` 或 `pg`
  - `KUP_PG_DSN`：Postgres 连接串（示例：`postgres://kupool:kupool@localhost:5432/kupool?sslmode=disable`）
  - `KUP_MQ`：`memory` 或 `rabbit`
  - `KUP_MQ_URL`：RabbitMQ URL（如 `amqp://guest:guest@localhost:5672/`）
  - `KUP_MQ_QUEUE`：队列名（如 `kupool_submissions`）

测试指南
- 并发客户端：启动多个客户端进程，观察服务端并发授权与广播日志。
- 正常流程：确认“收到 job 立即提交”“每秒提交”和“每分钟守护提交”日志。
- 错误用例：
  - 错误 `result` → 返回 `Invalid result`；
  - 过频提交 → 返回 `Submission too frequent`；
  - 重复 `client_nonce` → 返回 `Duplicate submission`；
  - 非最新 `job_id`：若存在历史且未过期，按历史校验；否则 `Task does not exist`；
  - 过期任务（开启 `KUP_EXPIRE`）→ 返回 `Task expired`。
- 统计验证：在 Postgres 中查询某用户在某分钟的 `submission_count`。

项目结构
- 核心目录：
  - `cmd/kupool-server`：服务端入口
  - `cmd/kupool-client`：客户端入口
  - `app/server`：服务端应用层（协调器、监听器、状态）
  - `app/client`：客户端应用层（事件驱动 Run）
  - `tcp`：TCP 客户端与服务器实现、帧协议
  - `protocol`：请求/响应与参数编码
  - `stats`：统计存储（Postgres）
  - `mq`：消息队列（内存与 RabbitMQ）

故障排查
- 无法连接服务端：检查 `-addr` 与防火墙；确认服务端日志输出。
- 无 RabbitMQ/PG：使用内存队列与内存统计运行（设置 `KUP_MQ=memory`, `KUP_STORE=memory`）。
- 结果错误：使用在线工具校验 `SHA256(server_nonce + client_nonce)`。

附注
- 本项目遵循面试题规范：可运行、可测试、可扩展；核心场景优先保证正确性与可读性。
