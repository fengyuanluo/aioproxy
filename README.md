# AIOPROXY

AIOPROXY 是一个 Go 编写的单实例代理聚合器，对外提供同端口 HTTP/SOCKS5 混合代理入口，插件导入上游代理候选并由主体负责验活、调度、session 绑定、失败淘汰和持久化。

当前支持两类用户名定向能力：

- `plugin`：按插件名筛选候选，例如 `aio~plugin=fpl`
- `region`：按代理出口国家码筛选候选，例如 `aio~region=US`

地区路由依赖候选在更新时通过 `validation.strategy=ip_api_country` 拿到 `countryCode`。

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

## 用户名示例

```text
aio
aio-job-001-30m
aio~plugin=fpl
aio~region=US
aio~plugin=singbox~region=HK~session=job-001~ttl=30m
```

更多规则见 [`docs/sessions.md`](docs/sessions.md)。

## 安全提示

示例配置默认监听 `127.0.0.1:1080`，默认认证为 `aio/change-me`。如果将代理暴露到非本地地址，必须修改默认密码。
