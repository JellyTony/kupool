## 问题与目标
- 现状：`Run` 顶层先阻塞读，再进入 `select`；导致定时器分支无法被及时触发。
- 目标：收到 `job` 立即提交一次；随后严格满足“最多 1 次/秒，最少 1 次/分钟”的提交节律；正确处理服务端响应。

## 重构方案
1. 事件驱动循环：建立 `readCh`（接收帧）与 `errCh`（读错误），启动独立读协程持续 `cli.Read()` 推送到 `readCh`。
2. 主循环 `select` 同时监听：`ctx.Done()`、`errCh`、`readCh`、`ticker.C(1s)`、`minuteTicker.C(1m)`。
3. 立即提交：`readCh` 收到 `job` 后，更新本地 `jobID/serverNonce` 并立即构造并发送 `submit`，更新 `lastSubmit`。
4. 响应处理：优先尝试按 `Response` 解码（`resp.ID!=0` 视为响应），打印 `submit ok/failed`；否则按 `Request` 处理 `job`。
5. 频率控制：
   - 每秒分支：`serverNonce!=""` 且 `time.Since(lastSubmit) >= 1s` 时提交；否则 `skip submit due to rate limit`。
   - 每分钟分支：若 `time.Since(lastSubmit) >= 1m` 则补一次提交（minute guard）。

## 影响范围
- 仅修改 `app/client/client.go:Run(ctx)` 内部逻辑；不改接口签名与外部行为。
- 服务端逻辑与接口保持不变，当前实现已满足分发与校验要求。

## 验证
- 编译通过 `go build ./...`。
- 运行联调：观察收到 `job` 后立即提交；随后定时分支稳定触发；响应日志正确输出。