# AIOPROXY

AIOPROXY 是一个 Go 编写的单实例代理聚合器，对外提供同端口 HTTP/SOCKS5 混合代理入口，插件导入上游代理候选并由主体负责验活、调度、session 绑定、失败淘汰和持久化。

## 快速入口

- 快速开始：[`docs/quickstart.md`](docs/quickstart.md)
- 配置样例：[`examples/config.yaml`](examples/config.yaml)
- 拓扑图：[`docs/topology.md`](docs/topology.md)
- ADR：[`docs/adr/`](docs/adr/)

## 命令

```bash
aioproxy check -c config.yaml
aioproxy serve -c config.yaml
```

## 安全提示

示例配置默认监听 `127.0.0.1:1080`，默认认证为 `aio/change-me`。如果将代理暴露到非本地地址，必须修改默认密码。
