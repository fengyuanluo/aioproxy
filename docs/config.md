# 配置说明

AIOPROXY 只读取 YAML 配置，不做环境变量展开。推荐从 `examples/config.yaml` 复制出 `config.yaml` 后修改。

## server

- `server.listen`：HTTP/SOCKS5 混合代理监听地址。默认示例为 `127.0.0.1:1080`，如需给局域网设备使用再显式改成 `0.0.0.0:1080` 或具体内网地址。

## admin

- `admin.listen`：只读 Admin API 监听地址。loopback 地址可不配置 token。
- `admin.token`：当 `admin.listen` 是非 loopback 地址时必填；请求使用 `Authorization: Bearer <token>`。

## auth

- `auth.enabled`：是否启用代理入口认证。示例默认启用。
- `auth.username`：唯一静态 credential，不能包含 `-`。
- `auth.password`：唯一静态密码。示例密码 `change-me` 仅用于本机 quickstart。

## scheduler

- `scheduler.policy`：非 session 请求调度策略。可选 `random` 或 `round_robin`。session 绑定不是这里的调度策略，而是由用户名表达式显式触发。

## session

- `session.default_ttl`：未显式写 TTL 的 session 滑动过期时间。
- `session.max_ttl`：允许请求的最大 session TTL，超过会 clamp 到该值。

## validation

- `validation.strategy`：验活策略。`http_status` 为传统 HTTP 状态码验活；`ip_api_country` 为通过代理请求 ip-api，同时拿存活结果和 `countryCode`。
- `validation.url`：更新时验活 URL。默认 HTTP `generate_204`，不验证 TLS 行为。
- `validation.success_status`：验活成功 HTTP 状态码列表。
- `validation.timeout`：单个候选验活超时。
- `validation.concurrency`：刷新时并发验活 worker 数。
- `validation.tls_insecure`：HTTPS 验活时是否跳过 TLS 校验。
- `validation.skip_validation`：仅用于本地实验/测试；正常运行应为 `false`。

当 `validation.strategy=ip_api_country` 时，建议使用 `http://ip-api.com/json/?fields=status,message,country,countryCode,query`。该模式下返回 `countryCode` 的候选才会入池，后续用户名可按国家码路由。

## runtime_failure

- `runtime_failure.max_failures`：运行时失败累计到该值后候选退出调度。
- `runtime_failure.retry_attempts`：单次请求首个候选失败后，允许在同一路由过滤结果内继续尝试的额外候选数；`0` 表示关闭请求级换源重试。
- `runtime_failure.early_failure_window`：早期零字节关闭窗口，命中后计为候选失败。

## storage

- `storage.data_dir`：持久化 pool 和 snapshots 的目录。
- `storage.snapshot_retention`：每个来源保留的 snapshot 数。

## logging

- `logging.file`：默认文件日志路径。
- `logging.level`：`debug` / `info` / `warn` / `error`。debug 日志可能包含敏感原文。
- `logging.format`：`text` 或 `json`。
- `logging.rotation.max_size`：单文件轮转大小。
- `logging.rotation.max_backups`：轮转文件保留数。
- `logging.rotation.max_age_days`：按天保留，0 表示不按年龄清理。
- `logging.rotation.compress`：是否压缩轮转日志。

## lifecycle

- `lifecycle.grace_period`：SIGINT/SIGTERM 后等待现有连接收尾的最长时间，到期后强制关闭仍活动连接。

## refresh

- `refresh.jitter_ratio`：启动后定时刷新 jitter 比例。启动即时刷新不加 jitter。

## plugins.fpl

存在 `plugins.fpl` 块即启用 FPL。省略 `url` 时使用内置 all-list URL。HTTP 与 SOCKS5 导入，SOCKS4 跳过。

## plugins.fofa

- `base_url`：FOFA-compatible API 地址。
- `key`：API key。为空则 FOFA 不激活。
- `size`：每个查询单次获取量。
- `refresh_interval`：FOFA 插件级刷新周期。
- `queries`：自定义查询列表。为空时使用内置 HTTP / SOCKS5 默认查询。

## plugins.singbox

- `refresh_interval`：sing-box 插件级刷新周期。
- `sources`：订阅/配置来源列表。支持 `url`、`file`、`inline` 类型。每个可转换节点作为独立 candidate，无法转换或无法启动的节点进入 import report 的 skip 统计。
