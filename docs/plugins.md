# 插件

插件统一输出候选代理和导入报告；主体负责去重、验活、入池、调度、session 绑定、运行时失败淘汰、持久化和 Admin API 可观测。

## FPL

来源为 proxifly/free-proxy-list all list。导入：

- `http://` / `https://`：HTTP upstream candidate。
- `socks5://` / `socks://`：SOCKS5 upstream candidate。
- `socks4://`：跳过并计入 import report。

## FOFA

使用 FOFA-compatible `/api/v1/search/all`，参数包括 `key`、`qbase64`、`fields`、`size`、固定第一页。默认查询：

- SOCKS5：`protocol=="socks5" && banner="Method:No Authentication"`
- HTTP：常见 Proxy-Authenticate / Squid / tinyproxy / 3proxy banner 线索。

结果按 `fields` 顺序映射，候选仍需主体更新时验活后才入池。

## sing-box

sing-box 依赖隔离在插件内。支持 remote URL、local file、inline 输入；支持 sing-box native outbounds、Clash-like `proxies`、base64 share-link list、single share link。每个成功转换并启动的节点作为独立 candidate。

不支持或启动失败的节点不会拖垮整个插件；它们进入 import report 的 `skip_reasons`。只要至少一个节点成功，插件可参与调度。
