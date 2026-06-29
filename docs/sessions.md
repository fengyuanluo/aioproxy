# Session 绑定

Session 只通过代理认证用户名显式触发，不是调度策略。

格式：

```text
<credential>
<credential>-<session>
<credential>-<session>-<ttl>
<credential>-fast
<credential>-<session>-fast
<credential>-<session>-<ttl>-fast
<credential>~plugin=<plugin>
<credential>~region=<CC>
<credential>~fast=true
<credential>~plugin=<plugin>~region=<CC>~session=<session>~ttl=<ttl>
```

示例：

```text
aio
aio-job001
aio-job-001
aio-job-001-30m
aio-fast
aio-job-001-fast
aio~plugin=fofa
aio~region=US
aio~fast=true~plugin=fpl
aio~plugin=singbox~region=HK~session=job-001~ttl=30m
```

规则：

- credential 不能包含 `-`。
- 不含 `~` 时完全沿用旧的 `-session[-ttl]` 语法。
- legacy `-fast` 只识别**最末尾后缀**，表示该请求只在 fast 池中调度。
- 含 `~` 时使用 KV 路由语法；当前支持 `plugin`、`region`、`session`、`ttl`、`fast`。
- `fast` 当前只接受 `true`，例如 `aio~fast=true~plugin=fpl`。
- `plugin` 统一按小写匹配插件名（如 `fpl` / `fofa` / `singbox`）。
- `region` 统一按大写国家码匹配（如 `US` / `JP` / `HK`）。
- `fast` 的速度来源是最近一次成功 validation 的校验耗时。
- 第一个 `-` 后是 session 表达式。
- 最后一段能解析成 duration 时作为 TTL。
- TTL 是滑动过期，每次使用刷新。
- `ttl` 只能和 `session` 一起出现。
- 当同时指定 `plugin` 与 `region` 时，先按插件过滤，再按国家码过滤，取交集；如果同时指定 `fast`，则在路由过滤后再截取前 `scheduler.fast_pool_percent` 的最快候选。
- fast 池截取按百分比向下取整，但只要命中候选非空，至少保留 1 个。
- 过滤后如果没有可用候选，请求直接失败，不会回退全局池。
- 请求 TTL 超过 `session.max_ttl` 时 clamp，不拒绝。
- 被绑定候选退出候选池后，下次请求自动重新绑定。
- session 不持久化，重启后清空。
