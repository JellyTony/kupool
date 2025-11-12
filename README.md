# KuPool v1

## 项目概述

- 项目名称：KuPool（基于 TCP 的消息系统）
- 当前版本：v1（参考 `examples/main.go:12`）
- 主要目的：提供一套高性能、可插拔后端（存储与队列）的作业广播与提交校验系统。服务端按固定间隔广播任务（`job_id` 与 `server_nonce`），客户端计算 `sha256(server_nonce + client_nonce)` 并提交，服务端完成去重、限频与结果校验后再异步写入统计与消息队列。
- 核心功能：
  - 连接握手与授权（authorize）
  - 周期性任务广播（job）
  - 提交校验与异步事件发布（submit）
  - 可插拔后端：`stats`（内存/PG）与 `mq`（内存/RabbitMQ）
- 适用用户与场景：
  - 构建“工作分发-提交校验”类系统（如 PoW 模拟、压测与教学）
  - 需要通过 MQ 与存储统计提交事件的系统
  - TCP/WS 连接管理与消息编解码的参考实现

## 安装指南

- 系统要求：
  - Go 1.25+（本机为 `go1.25.1`）
  - macOS 或 Linux
  - 可选依赖：PostgreSQL（统计）、RabbitMQ（事件）
- 获取与构建：
  - `go build ./cmd/kupool-server`
  - `go build ./cmd/kupool-client`
- 运行（验证过）：
  - 启动服务端（TCP）：
    - `./bin/kupool-server -addr 127.0.0.1:8080 -interval 3s -store memory -mq memory`
    - 运行日志示例：
      ```
      INFO started listen="127.0.0.1:8080" module=tcp.server
      INFO broadcast job job_id=1 nonce=<hex> sessions=0 module=app.coordinator
      ```
  - 运行单元测试：
    - `go test ./app/server -v`
    - 关键日志包含广播、授权、提交限频/去重与关闭流程（已验证通过）。
- 常见安装问题：
  - RabbitMQ 未运行或队列权限不足：请确认 `-mq_url` 与 `-mq_queue` 有效；连接失败会在后台消费者处报错。
  - PostgreSQL DSN 不正确：`-pg_dsn` 需为完整 DSN；首次连接会自动建表（`stats/pg.go:19-27`）。
  - 客户端地址解析异常（`cmd/kupool-client`）：当前 TCP 客户端在 `tcp/client.go:62-65` 使用 `url.Parse` 校验地址，传入 `host:port` 会报错；该问题已在“常见问题”章节给出规避建议。

## 模块说明

- 主要模块与功能：
  - `app/server`：应用服务端（协调器、接入器、监听器、状态）
    - `server.go:28-37` 启动 MQ 订阅与广播主循环
    - `acceptor.go:22-59` 授权握手（authorize）
    - `coordinator.go:37-51` 广播循环；`rotateJob` 与 `broadcastJob`
    - `listener.go:49-88` 提交校验（去重、限频、过期、结果校验）
  - `app/client`：示例客户端（接收 `job`，周期提交 `submit`）
  - `tcp` / `websocket`：协议适配与连接管理
    - `tcp/server.go:50-115` TCP 服务端实现
    - `websocket/server.go:47-113` WebSocket 服务端实现
  - `protocol`：消息编解码与参数定义（`protocol/message.go`）
  - `mq`：消息队列后端（内存/RabbitMQ）
  - `stats`：统计后端（内存/PostgreSQL）
  - `logger`：日志封装与滚动写入
  - `events`：事件结构定义
- 模块交互关系图：
  ```text
  +-----------+       job(broadcast)       +----------------+
  | Coordinator|--------------------------->|  Clients       |
  |  (server) |                            | (tcp/ws)       |
  +-----+-----+                             +--------+------+
        |  authorize                               ^
        v                                           |
  +-----+-----+  submit(result, nonce)      +-------+------+
  | Listener  |<----------------------------| Client logic |
  +-----+-----+                             +--------------+
        |
        | Publish(SubmitEvent)      Increment(username, minute)
        v                                    v
  +-----+-----+                       +------+------+
  |   MQ     |----------------------->|   Stats     |
  +-----------+                       +-------------+
  ```
- API 接口（JSON）：
  - 请求结构：`{"id":<number|null>,"method":"authorize|submit|job","params":{...}}`（`protocol/message.go:7-11`）
  - 响应结构：`{"id":<number>,"result":true|false,"error":<string?>}`（`protocol/message.go:13-17`）
  - `authorize` 参数：`{"username": "<string>"}`（`protocol/message.go:19-21`）
  - `job` 参数：`{"job_id": <int>, "server_nonce": "<hex>"}`（`protocol/message.go:23-26`）
  - `submit` 参数：`{"job_id": <int>, "client_nonce": "<hex>", "result": "<hex>"}`（`protocol/message.go:28-32`）
  - 校验规则（服务端）：
    - 频率限制：同一会话提交间隔 < 1s 拒绝（`listener.go:66-69`）
    - 去重：同 `job_id` 下重复 `client_nonce` 拒绝（`listener.go:70-77`）
    - 结果校验：`sha256(server_nonce + client_nonce)` 十六进制需与 `result` 一致（`listener.go:78-82`）
    - 历史任务：若客户端提交旧 `job_id`，从 `history` 取 `server_nonce` 并校验过期（`listener.go:56-65`；`coordinator.go:54-61`）

## 使用示例（已验证）

- WebSocket 演示（`examples/mock`）：
  - 启动 WS 服务端：
    - `go run ./examples/main.go mock_srv -p ws -a :8000`
    - 期望输出：`INFO started listen=":8000" module=ws.server`
  - 启动 WS 客户端：
    - `go run ./examples/main.go mock_cli -p ws -a ws://localhost:8000`
    - 期望输出（示例）：
      ```
      WARN <id> receive message [hello from server ]
      WARN <id> receive message [hello from server ]
      ... 共 5 条
      ```
- TCP 服务端（应用）：
  - 已成功运行：`./kupool-server -addr 127.0.0.1:8080 -interval 3s -store memory -mq memory`
  - 广播日志：`INFO broadcast job job_id=1 nonce=<hex> sessions=0`
- 单元测试（提交流程）：
  - `go test ./app/server -v`，包含并发客户端、去重与限频校验（均通过）。

## 配置说明

- 命令行参数（`cmd/kupool-server/main.go:15-23`）：
  - `-addr`：监听地址（如 `127.0.0.1:8080`）
  - `-interval`：任务广播间隔（如 `30s`）
  - `-store`：统计后端 `memory|pg`
  - `-pg_dsn`：PostgreSQL DSN（启用 `pg` 必填）
  - `-mq`：消息队列后端 `memory|rabbit`
  - `-mq_url`：RabbitMQ 连接 URL
  - `-mq_queue`：RabbitMQ 队列名
- 环境变量（优先于参数，`cmd/kupool-server/main.go:24-31`）：
  - `KUP_ADDR`、`KUP_INTERVAL`、`KUP_STORE`、`KUP_PG_DSN`、`KUP_MQ`、`KUP_MQ_URL`、`KUP_MQ_QUEUE`
- 日志级别：
  - `KUP_LOG_LEVEL=trace|debug|info|warn|error`（`logger/logger.go:104-111`）
- 推荐配置：
  - 开发环境：`-store memory -mq memory -interval 3s`（便于观察广播与提交）
  - 生产环境：`-store pg -mq rabbit`，开启持久化与至少一次投递；合理设置 `-interval` 与队列持久化。

## 开发指南

- 项目结构（目录摘录）：
  - `app/server/`：业务服务端（授权、广播、提交与状态）
  - `app/client/`：示例客户端逻辑
  - `tcp/`、`websocket/`：两种协议实现（服务端/客户端/连接）
  - `protocol/`：请求/响应/参数与编解码
  - `mq/`：消息队列（内存/RabbitMQ）
  - `stats/`：统计（内存/PostgreSQL）
  - `examples/`：WS/TCP 演示命令
- 添加新模块示例：新增 MQ 后端
  - 定义满足接口的类型（`app/server/types.go:40-44`）：实现 `Publish/Subscribe/Close`
  - 在 `cmd/kupool-server/main.go` 中增加分支装配该后端
  - 验证：运行服务端，观察 `Subscribe` 通道是否收到事件
- 添加新存储后端：实现 `StatsStore.Increment`（`app/server/types.go:36-38`），并在启动处装配
- 调试与日志：统一使用 `logger`，通过环境变量控制级别
- 测试方法：
  - 单元测试：`go test ./app/server -v`
  - 可补充：对 MQ/PG 后端各自的集成测试（仓库已有 `mq/*_test.go` 与 `stats/*_test.go`）

## 常见问题与故障排除

- `./kupool-client` 连接报错：`parse "127.0.0.1:8080": first path segment in URL cannot contain colon`
  - 原因：`tcp/client.go:62-65` 使用 `url.Parse` 校验地址，裸 `host:port` 会被判为非法。
  - 现状：该校验与后续 `net.DialTimeout` 接收的地址格式不一致。
  - 建议：优先使用 `examples/mock` 的 WS 客户端进行端到端演示；或将 TCP 客户端的地址校验逻辑改为仅检查非空并允许 `host:port`（开发改动）。
- 提交频率过高：日志出现 `Submission too frequent`（`listener.go:66-69`）——请在客户端侧控制最小 1s 的提交间隔。
- 重复提交：同 `job_id` 下重复 `client_nonce` 会被拒绝（`listener.go:70-77`），请生成唯一随机 `client_nonce`。
- 任务过期或不存在：提交旧 `job_id` 且超过 `expireAfter` 或历史中不存在会被拒绝（`listener.go:56-65`）。
- RabbitMQ 消费异常：检查队列是否已创建、是否设置持久化；确认网络连通与认证信息。
- PostgreSQL 计数异常：确认时区与 `date_trunc('minute', ...)` 的行为（`stats/pg.go:30-39`）。

## 版本与许可

- 版本：v1（示例）
- 许可：遵循仓库默认开源许可（如需）。
