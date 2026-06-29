# Admin API

Admin API 是只读接口。默认监听 `127.0.0.1:1081` 时不需要 token；非 loopback 监听必须配置 `admin.token`。

## 鉴权

```bash
curl -H 'Authorization: Bearer <token>' http://host:1081/health
```

## 端点

- `GET /health`：整体状态、pool 数、session 数。插件 degraded、无 active plugin 或空池会导致 degraded。
- `GET /stats`：pool、session、插件状态汇总。
- `GET /pool`：基础候选信息，不返回 proxy password、FOFA key、完整订阅 URL 或 raw node。若候选通过 `ip_api_country` 验活入池，还会包含 `CountryCode` / `Country`。支持分页/过滤参数：`limit`（默认 1000，最大 5000）、`offset`、`source`、`status`、`protocol`；响应头包含 `X-Pool-Matched`、`X-Pool-Returned`、`X-Pool-Limit`、`X-Pool-Offset`。
- `GET /plugins`：插件状态和 import reports。
- `GET /snapshots`：已保留 snapshot 文件列表。

## `GET /pool` 视图

返回的是基础运行信息视图，常见字段包括：

- `ID`
- `Fingerprint`
- `Protocol`
- `Host`
- `Port`
- `Source`
- `Name`
- `Status`
- `FailureCount`
- `LastValidation`
- `CountryCode`
- `Country`

其中 `CountryCode` / `Country` 只有在候选经过 `ip_api_country` 验活并成功返回国家信息时才会出现。

Admin API 不提供刷新、删除、修改等 mutation。真实深度调试看文件日志；debug 日志可能包含敏感原文。
