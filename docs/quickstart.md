# 快速开始

```bash
cp examples/config.yaml config.yaml
aioproxy check -c config.yaml
aioproxy serve -c config.yaml
```

默认代理监听 `127.0.0.1:1080`，Admin API 监听 `127.0.0.1:1081`，代理账号密码为 `aio/change-me`。

HTTP proxy 测试：

```bash
curl -x http://aio:change-me@127.0.0.1:1080 http://example.com/
```

SOCKS5 proxy 测试：

```bash
curl --socks5-hostname aio:change-me@127.0.0.1:1080 http://example.com/
```

Session 用户名示例：

```bash
curl -x http://aio-job-001-30m:change-me@127.0.0.1:1080 http://example.com/
```

Admin API：

```bash
curl http://127.0.0.1:1081/health
curl http://127.0.0.1:1081/stats
curl http://127.0.0.1:1081/pool
curl http://127.0.0.1:1081/plugins
```

示例配置默认启用 FPL 插件，启动时会访问 FPL 默认源。若 FPL 失败、无 active plugin、候选池为空或插件导入 0 个可用代理，health 会显示 degraded；候选池为空时代理请求会 fail fast。
