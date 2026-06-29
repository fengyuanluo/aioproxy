# 代理使用

AIOPROXY 在同一个端口上自动识别 HTTP proxy 与 SOCKS5。

## HTTP proxy

```bash
curl -x http://aio:change-me@127.0.0.1:1080 http://example.com/
```

## HTTP CONNECT

```bash
curl -x http://aio:change-me@127.0.0.1:1080 https://example.com/ -I
```

## SOCKS5

```bash
curl --socks5-hostname aio:change-me@127.0.0.1:1080 http://example.com/
```

## 速度优先路由

```bash
curl -x http://aio-fast:change-me@127.0.0.1:1080 http://example.com/
curl -x http://aio~fast=true~plugin=fpl:change-me@127.0.0.1:1080 http://example.com/
```

说明：

- `aio-fast`：直接在全局可用候选中取校验速度最快前百分比的子集再调度。
- `aio~fast=true~plugin=fpl`：先按 `plugin=fpl` 过滤，再从命中的候选里取最快前百分比。
- 百分比由 `scheduler.fast_pool_percent` 控制，默认 `5`。
- 只要命中候选非空，fast 子集至少保留 1 个；若路由后为空，仍直接失败，不回退全局池。

## 空池行为

当 Candidate Pool 为空时，请求立即失败：HTTP 返回 service-unavailable 风格错误，SOCKS5 返回 general failure。
