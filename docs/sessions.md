# Session 绑定

Session 只通过代理认证用户名显式触发，不是调度策略。

格式：

```text
<credential>
<credential>-<session>
<credential>-<session>-<ttl>
```

示例：

```text
aio
aio-job001
aio-job-001
aio-job-001-30m
```

规则：

- credential 不能包含 `-`。
- 第一个 `-` 后是 session 表达式。
- 最后一段能解析成 duration 时作为 TTL。
- TTL 是滑动过期，每次使用刷新。
- 请求 TTL 超过 `session.max_ttl` 时 clamp，不拒绝。
- 被绑定候选退出候选池后，下次请求自动重新绑定。
- session 不持久化，重启后清空。
