## 现状
- 已有自研 `logger` 封装，基于 `logrus`，支持按天滚动与通用设置。
- 服务端在 `cmd/kupool-server/main.go:31` 仅调用 `logger.SetLevel`；客户端与服务端存在少量标准库 `log.Fatal` 混用。
- 代码入口：`cmd/kupool-server/main.go:15`、`cmd/kupool-client/main.go:10`、`examples/main.go:14`。

## 目标
- 在所有入口统一初始化日志为 JSON 格式，输出到 stdout，级别由环境或 flag 控制。
- 增加 `docker-compose.yml`，提供 `kupool-server` 容器化运行（可选 `postgres`、`rabbitmq` 依赖），以及可选 `kupool-client`。

## 日志改动方案
- 服务端：在 `cmd/kupool-server/main.go:23-32` 的参数解析与环境读取之后，新增调用：
  - `logger.Init(logger.Settings{ Format: "json", Level: os.Getenv("KUP_LOG_LEVEL") })`
  - 保持输出到 stdout（不设置 `Filename`），适配容器化日志收集。
  - 若初始化失败，使用 `logger.WithError(err).Fatal("logger init failed")` 终止。
- 客户端：在 `cmd/kupool-client/main.go:12-20` 解析参数后新增相同的 `logger.Init`；将 `log.Fatal` 改为 `logger.WithError(err).Fatal(...)` 保持一致。
- 示例程序：在 `examples/main.go:18-24` 初始化为 JSON 格式，便于示例与本体一致。
- 若需要文件滚动，可替换为 `logger.InitDailyRolling` 并设置 `Filename` 与 `RollingDays`，默认不启用。

## Docker Compose 方案
- 新增 `docker-compose.yml`，包含：
  - `kupool-server` 服务：
    - 构建：多阶段 `Dockerfile`（Go 1.22 builder + 精简运行时），产出 `/app/kupool-server`。
    - 端口：`8080:8080`。
    - 环境：`KUP_ADDR`, `KUP_INTERVAL`, `KUP_STORE`, `KUP_PG_DSN`, `KUP_MQ`, `KUP_MQ_URL`, `KUP_MQ_QUEUE`, `KUP_LOG_LEVEL`。
    - `depends_on`：按需启用 `postgres` 或 `rabbitmq`。
  - `postgres`（可选）：`postgres:16-alpine`，暴露 `5432`，挂载数据卷。
  - `rabbitmq`（可选）：`rabbitmq:3-management`，暴露 `5672/15672`。
  - `kupool-client`（可选，通过 `profiles: [client]`）：构建客户端镜像，启动命令通过 `command` 指定 `-addr` 与 `-username`。
- 新增两个 `Dockerfile`：
  - 服务端：在项目根或 `docker/server.Dockerfile`，多阶段构建二进制并以最小镜像运行。
  - 客户端：在项目根或 `docker/client.Dockerfile`，同样多阶段构建。
- 日志输出：容器内输出到 stdout，`docker compose logs -f server` 可看到 JSON 格式日志。

## 验证步骤
- 构建：`docker compose build`。
- 启动（仅服务端）：`docker compose up -d kupool-server`。
- 查看日志：`docker compose logs -f kupool-server`，确认为 JSON。
- 启动依赖（如使用 `pg` 或 `rabbit`）：`docker compose up -d postgres rabbitmq` 并设置相关环境。

## 安全与约定
- 不引入新依赖库，复用现有 `logger` 封装；不提交任何敏感信息。
- 环境变量通过 Compose 传入，默认不创建 `.env` 文件（如需可后续添加）。

## 交付内容
- 在三个入口文件新增 JSON 日志初始化。
- 新增 `docker-compose.yml` 与两个 `Dockerfile`（server、client），支持本地构建与运行。