## 总览
- 目标：在现有 `kupool` 框架上实现符合题目协议的 TCP 长连接消息系统，包含服务端、客户端、消息处理器（可选）与测试和 README。
- 语言：Go（macOS/Linux 可编译运行）。
- 复用：直接复用已有 TCP 抽象与事件循环（如 `tcp/server.go:50-115`、`channel.go:116-145`、`tcp/client.go:60-95`）。

## 架构设计
- 协议：应用层统一使用 JSON 文本，承载于现有帧协议（`opcode: OpBinary` + 长度前缀，见 `tcp/connection.go:75-84`）。
- 组件：
  - Server：维护会话、任务广播、提交校验、速率限制、重复 nonce 检测。
  - Client：鉴权、接收任务、计算 SHA256(server_nonce+client_nonce) 并按频率提交。
  - StatsStore：统计存储抽象，默认 memory，可替换 PostgreSQL。
  - JobHistory：可选维护 `job_id ↔ server_nonce` 历史（加分）。
  - MessageProcessor：异步消费提交事件，写统计，默认 memory 队列，可替换 RabbitMQ（加分）。
- 并发模型：
  - 每连接一个读循环（已有，参考 `channel.go:116-145`）。
  - 服务端一个任务广播协程（每 30s 更新 `server_nonce`，递增 `job_id`）。
  - 消息处理器一个或多个 worker 协程异步刷写统计。

## 数据模型
- 通用消息：
  - Request：`{"id":<number|null>, "method":"...", "params":{...}}`
  - Response：`{"id":<number>, "result":<bool>, "error":<string,omitempty>}`
- 会话：`Session{channel_id, username, latest_job_id, latest_server_nonce, last_submit_at, used_nonces: map[job_id]set<string>}`
- 统计：`SubmissionStat{username, minute_ts, submission_count}` 对应表结构。
- 任务历史（可选）：`JobHistory{job_id, server_nonce, created_at, expires_at(optional)}`

## 服务端实现
- 接入点：
  - 复用 `tcp.NewServer` 与事件循环（`tcp/server.go:64-115`）。
  - 使用自定义 `Acceptor` 完成首次消息鉴权（替换 `defaultAcceptor`，见 `tcp/server.go:178-184`）。
- 鉴权流程：
  - 在 `Acceptor.Accept(conn, timeout)` 中读取首帧（`conn.ReadFrame()`），解析 JSON：必须为 `method:"authorize"`，`params.username` 非空，返回 `channel_id` 与绑定 `username`；否则返回错误并关闭（参考 `examples/mock/server.go:41-55` 读取首帧模式）。
- 会话管理：
  - 维护 `map[channel_id]Session` 与 `map[username]channel_id`（确保单用户多会话允许，但每会话各自维护最新 nonce；题目要求“每个会话只维护最新 server_nonce”）。
- 任务广播：
  - 每 30s：生成新 `server_nonce`（安全随机），递增全局 `job_id`，向所有在线会话 `Push`：`{"id":null,"method":"job","params":{"job_id":J,"server_nonce":S}}`。
  - 更新每会话 `latest_job_id` 与 `latest_server_nonce`；可选写入 `JobHistory`。
- 提交处理：
  - 在 `MessageListener.Receive(agent, payload)` 中解析 `submit`：
    - 结构校验：存在 `id`、`params.job_id`、`params.client_nonce`、`params.result`。
    - 频率限制：每会话 1 秒最多一次（比较 `last_submit_at`）。违反返回：`{"id":X,"result":false,"error":"Submission too frequent"}`。
    - 任务校验：
      - `job_id` 必须等于会话 `latest_job_id`（或存在于历史），否则 `Task does not exist`；若过期（可选）则 `Task expired`。
      - `server_nonce` 取自会话（或历史）与 `client_nonce` 拼接计算 `sha256`；不匹配返回 `Invalid result`。
      - 检查重复 `client_nonce`（会话的该 `job_id` 集合中存在则 `Duplicate submission`）。
    - 成功：返回 `{"id":X,"result":true}`，并将事件异步发布给 `MessageProcessor` 写统计（按分钟聚合）。
- 恢复：
  - 重启后：
    - 会话内存丢失，依赖新的鉴权建立；
    - 统计由 `StatsStore` 从 DB 持久；
    - 可选：从 `JobHistory` 恢复最近任务窗口（用于校验历史提交）。

## 客户端实现
- 复用 `tcp.NewClient` 与 `Dialer` 机制（`tcp/client.go:97-101`）。
- 首次握手：自定义 `Dialer.DialAndHandshake`：连接后立即发送首帧 JSON：
  - `{"id":1,"method":"authorize","params":{"username":"admin"}}`（或传入的用户名）。
- 任务接收：
  - 监听服务端 `job` 消息，更新本地 `server_nonce`、`job_id`。
- 结果提交：
  - 生成随机 `client_nonce`（安全随机十六进制/字母数字）。
  - 计算 `sha256(server_nonce + client_nonce)`，按频率规则提交：
    - 至多 1 次/秒：通过 `time.Ticker(1s)` 控制。
    - 至少 1 次/分钟：若未提交则在 60s 保底触发一次。
  - 序列化发送 `{"id":<next_id>,"method":"submit","params":{...}}` 并等待响应。

## 统计存储
- 接口：`type StatsStore interface { Increment(username string, minute time.Time) error; Get(username string, minute time.Time) (int, error) }`
- 默认内存实现：`map[username]map[minute_ts]int`，用于测试。
- PostgreSQL 实现（可插拔）：
  - 表：`submissions(username VARCHAR(255), timestamp TIMESTAMP, submission_count INT)`。
  - 写入策略：`INSERT ON CONFLICT (username,timestamp) DO UPDATE SET submission_count=submissions.submission_count+1`。
  - 配置通过环境变量或配置文件选择存储后端。

## 消息处理器（加分）
- 接口：`type MessageQueue interface { Publish(SubmitEvent) error; Subscribe(ctx) (<-chan SubmitEvent); Close() error }`
- 默认内存队列：带缓冲 `chan SubmitEvent`；worker 从 `Subscribe` 消费并调用 `StatsStore.Increment`。
- RabbitMQ 实现（可选）：使用持久化队列，`SubmitEvent` 序列化为 JSON，开启 `ack`；支持服务端重启后消息不丢失。
- 恢复策略：
  - 内存队列：无持久化，重启后从 0 开始；统计不丢（DB 持久）。
  - RabbitMQ：消息持久化 + 消费确认，重启后自动恢复未确认消息。

## 错误与校验
- 统一服务端错误响应：`{"id":<id>,"result":false,"error":"..."}`。
- 错误条件覆盖：
  - `Task does not exist`：`job_id` 不匹配会话最新/历史。
  - `Task expired`（可选）：超过设定过期时间窗口。
  - `Invalid result`：SHA256 比对失败。
  - `Submission too frequent`：违反 1 秒最多一次。
  - `Duplicate submission`：同 `job_id` 重复 `client_nonce`。

## 测试计划
- 并发：
  - 启动服务端，模拟 N（≥5）客户端并发鉴权、接收任务与提交，确保全部成功。
- 错误验证：
  - 错误 `job_id`、过期任务、错误结果、超频提交、重复提交，分别断言服务端错误响应。
- 速率限制：
  - 连续 1s 内多次提交仅第一条成功，后续返回 `Submission too frequent`。
- 统计：
  - 提交后统计按分钟聚合自增；切换到 PostgreSQL 实现时断言持久化。
- 端到端：
  - 客户端从 `job` 到 `submit` 的完整闭环；跨分钟提交验证聚合。

## 代码组织
- 新增包：
  - `protocol/`：消息结构与编解码（JSON）。
  - `server/app/`：会话、任务、校验、广播、监听器与接受器。
  - `client/app/`：客户端逻辑与 `Dialer`。
  - `stats/`：`StatsStore` 内存与 PostgreSQL 实现。
  - `mq/`：`MessageQueue` 内存与 RabbitMQ 实现（可选）。
- 命令：
  - `cmd/kupool-server`：启动服务端（TCP 地址、存储/MQ 配置）。
  - `cmd/kupool-client`：启动客户端（用户名、服务器地址）。
- 保持与现有抽象一致：复用 `Server/Client/Channel/MessageListener/Acceptor` 接口（`server.go:16-38`、`server.go:85-97`）。

## README 与运行
- 构建：`go build ./cmd/kupool-server` 与 `go build ./cmd/kupool-client`。
- 运行：
  - 启动服务端：`./kupool-server -addr :8080 -store memory -mq memory`（或配置 PostgreSQL/RabbitMQ）。
  - 启动多个客户端：`./kupool-client -username admin -addr 127.0.0.1:8080`