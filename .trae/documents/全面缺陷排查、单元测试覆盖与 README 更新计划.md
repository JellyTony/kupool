## 目标与交付物
- 全面排查并修复隐藏问题，保证核心路径的健壮性与并发安全
- 为各包的关键函数与错误分支补齐单元测试与集成测试，覆盖率提升至可观的水平（优先 ≥80% 的核心包）
- 梳理并更新项目说明与使用文档到 `README.md`，补充配置项与测试说明

## 现有隐藏问题清单（需修复）
- `authorize` 响应 ID 可能为 `nil` 导致崩溃：`app/server/acceptor.go:69`
  - 现状：未校验 `req.ID` 是否为 `nil` 就解引用 `*req.ID`
  - 影响：恶意或异常客户端可能触发服务端 `panic`
- 内存状态存储历史记录的 `nonce` 错误：`stats/memory.go:73–74`
  - 现状：`LoadJobHistory` 为所有历史作业返回同一个 `latestNonce`
  - 影响：服务端从状态恢复历史任务时会使用错误的 `nonce` 校验，造成旧任务无法正确校验或误判
- 测试用例与当前签名不匹配（无法编译）：
  - `NewAppServer` 现为 7 参数，测试仍按 4 参数调用：`app/server/server_test.go:{62,76,89,105,120,139}`
  - `NewCoordinator` 现为 7 参数，测试仍按 5 参数调用：`app/server/acceptor_test.go:27`, `app/server/coordinator_test.go:13`
  - `Start` 现为 `Start(ctx context.Context) error`，测试中调用 `app.Start()`（未传参）
- 监听器对 `Agent` 做 `Channel` 类型断言，耦合较高：`app/server/listener.go:38`
  - 建议直接用 `Agent.ID()`，避免未来替换 `Agent` 实现时断言失败
- 广播/响应路径忽略错误：
  - 协调器广播忽略 `Encode/Push` 错误：`app/server/coordinator.go:93–101`
  - 监听器响应忽略 `Encode/Push` 错误：`app/server/listener.go:45–47`
  - 握手阶段写响应未处理错误：`app/server/acceptor.go:70–72`
  - 建议：记录并限速打印错误，必要时对会话做降级或踢除
- 客户端握手只写不读授权响应：`app/client/client.go:158–169`
  - 现状：发送 `authorize` 后直接认为成功，不读取服务端响应
  - 影响：授权失败的反馈延迟到后续读循环；建议在拨号器中读取一帧并校验 `Response`

## 修复方案
- `acceptor.go`：在写响应前校验 `req.ID`，缺失则返回 `unauthorized` 或不响应，避免 `panic`
- `stats/memory.go`：将 `jobHist` 由 `map[int]time.Time` 改为 `map[int]serverpkg.JobRecord`，在 `SaveJob` 保存 `nonce+createdAt`，`LoadJobHistory` 返回每个 Job 的正确 `nonce`
- `listener.go`：改用 `ag.ID()` 获取通道 ID，减少不必要断言
- 广播/响应统一错误处理：`Encode/Push/WriteFrame` 出错时记录 `error` 日志，必要时清理会话或统计错误计数
- 测试签名同步更新：调整所有 `NewAppServer/NewCoordinator/Start` 的调用参数与上下文传递

## 单元测试与集成测试计划
- 包 `protocol`：
  - 已有编解码测试；补充空 `id(nil)` 授权请求的边界用例
- 包 `mq`：
  - `memory`：并发发布/订阅、关闭后发布丢弃行为（`Publish` 在关闭后返回 `nil` 的语义验证）
  - `rabbitmq`：在提供 `KUP_MQ_URL` 时跑端到端发布/消费，验证 `Ack/Nack`
- 包 `stats`：
  - `memory`：`Increment/Get`、`SaveJob/LoadLatestJob/LoadJobHistory` 正确性（含历史 `nonce`）
  - `pg`：现有测试保留；补充 `StateStore` 模型的读写一致性与时间窗口查询
- 包 `tcp`：
  - `connection`：`ReadFrame/WriteFrame` 正常/错误路径（半包、非法长度）
  - `server`：`Push` 到不存在通道的错误分支；`Shutdown` 关闭所有通道
  - `client`：心跳 `ping`、读到 `OpClose` 的错误分支；拨号器读取授权响应
- 包 `kupool`（通道实现）：
  - `ChannelImpl.Push/writeloop`：批量写、`Flush` 调用、`closed` 后写入报错
  - `Readloop`：`OpPing` 回 `OpPong`、`OpClose` 结束、空 `payload` 跳过、并行回调的正确性
- 包 `app/server`：
  - `Acceptor.Accept`：正确授权、`opcode` 非 `OpBinary`、非法 JSON、空用户名、`req.ID=nil` 边界
  - `Coordinator`：`rotateJob` 保证 `job_id` 自增与 `nonce` 随机性；广播会话状态更新；`restore` 从状态加载
  - `Listener.handleSubmit`：成功、结果错误、过频、重复、非最新 `job_id`（历史存在/不存在）、过期任务
  - `AppServer.Start/Shutdown`：消费 MQ 增量、`Stop` 停止广播、重试关闭 MQ/Store、状态统计字段
- 集成测试：
  - 内存后端：启动服务端与若干客户端，覆盖成功与错误场景，验证 `/stats` 聚合结果
  - PG/MQ 后端（可选、带环境变量）：在 CI 条件允许下跑最小化端到端流

## 覆盖率与质量保障
- 使用 `make test-coverage` 生成覆盖率报告并核查核心包（`app/server`, `tcp`, `kupool`, `stats`, `protocol`）
- 使用 `make lint` 运行 `golangci-lint`，在修复中清理明显的 `errcheck`/并发数据竞争等问题

## README.md 更新计划
- 补充环境变量：增加 `KUP_HISTORY_WINDOW`（历史任务恢复窗口）说明
- 补充“单元测试与覆盖率”章节：`make test`、`make test-coverage`、如何在提供 `PG_DSN/KUP_MQ_URL` 时启用端到端测试
- 明确“错误用例与预期响应”列表，与服务端监听器实现一致（含过期任务）
- 增强“快速开始”与“故障排查”：加入常见构建/运行问题与解决方案
- 保留现有架构、协议与部署章节；微调引用路径（函数位置引用保持为 `file_path:line`）

## 交付与验证
- 修复上述问题并同步更新测试签名
- 完整运行：`make test` 与 `make test-coverage`；本地内存后端集成用例通过
- README 更新后进行一次开发者视角自检（按“构建与运行”“测试指南”逐步验证）

## 备注
- 以上变更全部在现有代码风格与依赖下进行，不引入新外部库
- 若需将覆盖率结果纳入 CI，可后续增加 GitHub Actions（本阶段不改动 CI）