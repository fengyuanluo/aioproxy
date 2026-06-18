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

## 空池行为

当 Candidate Pool 为空时，请求立即失败：HTTP 返回 service-unavailable 风格错误，SOCKS5 返回 general failure。
