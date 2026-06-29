# AIOPROXY 全链路性能、内存占用与可用性审计报告

**日期**：2026-06-29
**审计对象**：`/root/Coding/General/AIOPROXY` 当前工作树
**审计目标**：全面阅读全链路相关代码，深入研究性能问题、内存占用问题、可用性问题，并按重要性分级输出可执行报告。
**审计类型**：只读代码审计 + 本地验证/微基准/运行态 smoke。
**结论级别定义**：

- **P0 / Release-blocking**：默认路径或核心能力存在明确高影响风险；会造成严重性能退化、代理语义绕过、或生产不可接受的稳定性问题。
- **P1 / High**：高概率影响真实部署；需要进入近期修复台账。
- **P2 / Medium**：在规模、异常输入、暴露范围扩大或长时间运行后放大；建议排期治理。
- **P3 / Low / Hygiene**：不会立即阻断，但会影响可观测性、维护性或边界清晰度。

---

## 0. 当前结论总览

### 0.1 总体判断

当前 AIOPROXY 的基础功能链路已经具备完整结构：

1. `cmd/aioproxy/main.go` 调用 `app.Run()`；
2. `app.Run()` 完成配置加载、日志初始化、持久化池加载、Admin API、代理监听、插件刷新 loop；
3. 插件导入候选后进入 `validation.Validator` 更新时验活；
4. 验活通过的候选进入 `core.Pool`；
5. 请求经 HTTP/SOCKS5 混合入口进入 `proxy.Server`；
6. `SessionManager` + `Pool` 按 session/plugin/region 选候选；
7. `DialViaCandidate` 或 sing-box lazy dialer 建立上游连接；
8. 运行失败通过 `MarkFailure` 淘汰；
9. Admin API 暴露 health/stats/pool/plugins/snapshots；
10. graceful shutdown 持久化 pool/snapshot。

代码主路径能通过现有测试与 race 测试；但从性能、内存和可用性角度看，仍存在两类特别关键的问题：

- **默认 `random` 调度在大池场景是 O(N) 分配型热路径**，实测 10,000 候选下 5,000 次 pick 产生约 **12.5 GB TotalAlloc**，耗时 **19.23s**；同池 `round_robin` 为 **0 KB 分配、2.05ms**。这是当前最明确的 P0 性能问题。
- **sing-box `direct` outbound 会被导入为可用候选**。这意味着订阅/配置中出现 `direct` 节点时，AIOPROXY 会把“直连”当作代理候选调度，破坏“代理池只走上游代理”的直觉和产品边界。若用户依赖 AIOPROXY 隐藏真实出口，这属于 P0 可用性/语义安全问题。

此外，长时间运行后风险集中在：

- additive pool 不清理源中已消失候选，`Pool.items`/`Pool.dialers` 会随源 churn 增长；
- sing-box 验证峰值内存仍与候选数和验证并发强相关；
- tunnel 早期零字节判定会误伤空闲 CONNECT/SOCKS 隧道；
- 代理入口没有读超时/头大小上限/连接保护；
- refresh jitter 和 early failure window 缺少配置边界校验；
- 持久化和 Admin API 都是全量 JSON，池大后会形成 I/O、CPU、堆分配和响应延迟问题。

### 0.2 当前验证证据

本次审计实际执行并保存了以下证据：

```bash
GOFLAGS=-mod=readonly go test ./... -count=1
GOFLAGS=-mod=readonly go test -race ./... -count=1
GOFLAGS=-mod=readonly go run ./cmd/aioproxy check -c examples/config.yaml
GOFLAGS=-mod=readonly go test ./internal/proxy -run TestProxy300Concurrent -count=1 -v
GOFLAGS=-mod=readonly go vet ./...
```

结果：全部通过。完整输出保存在：

- `tmp/2026-06-29-audit-verification.log`

关键摘录：

```text
ok   github.com/aioproxy/aioproxy/internal/proxy  0.187s
ok   github.com/aioproxy/aioproxy/internal/storage 0.604s
ok   github.com/aioproxy/aioproxy/internal/validation 0.018s
ok   github.com/aioproxy/aioproxy/internal/proxy 1.390s  # race
AIOPROXY config check
active plugins: fpl
result: ok
=== RUN   TestProxy300Concurrent
--- PASS: TestProxy300Concurrent (0.22s)
PASS
```

补充运行态证据：

- 大池热路径微基准：`tmp/2026-06-29-pool-hotpath.log`
- 200 sing-box lazy direct 候选 idle RSS smoke：`tmp/2026-06-29-mem-smoke.log`
- direct outbound 被调度 smoke：`tmp/2026-06-29-direct-outbound-smoke.log`
- 错误 timing 配置仍通过 `check`：`tmp/2026-06-29-bad-timing-check.log`

---

## 1. 全链路代码覆盖范围

本次已阅读并纳入审计范围的代码/文档：

| 模块 | 文件 | 主要职责 |
|---|---|---|
| CLI | `cmd/aioproxy/main.go` | 入口，委派到 app |
| 应用生命周期 | `internal/app/app.go` | `check`/`serve`、启动、插件刷新、shutdown |
| 配置 | `internal/config/config.go` | YAML schema、默认值、配置检查 |
| 日志 | `internal/logging/logging.go` | slog + lumberjack 文件日志 |
| 核心类型 | `internal/core/types.go` | Candidate、ImportReport、PluginStatus、指纹 |
| 候选池 | `internal/core/pool.go` | pool、dialer map、调度、失败淘汰 |
| Session | `internal/core/session.go` | legacy/structured username、绑定、重绑 |
| 代理入口 | `internal/proxy/server.go` | TCP accept、HTTP/SOCKS5、retry、tunnel |
| 上游拨号 | `internal/proxy/upstream.go` | HTTP CONNECT、SOCKS5、URL 解析 |
| 验活 | `internal/validation/validation.go` | update-time validation、ip-api country |
| 持久化 | `internal/storage/storage.go` | pool.json、snapshots、atomic write |
| Admin | `internal/admin/admin.go` | health/stats/pool/plugins/snapshots |
| 插件接口 | `internal/plugins/plugins.go` | Plugin interface |
| FPL 插件 | `internal/plugins/fpl/fpl.go` | 文本代理列表导入 |
| FOFA 插件 | `internal/plugins/fofa/fofa.go` | FOFA-compatible API 查询 |
| sing-box 插件 | `internal/plugins/singbox/singbox.go` | 订阅/配置导入、lazy per-node dialer |
| 测试 | `internal/**/*_test.go` | 单测、race 依据、300 并发 gate |
| 脚本 | `scripts/stress_300.sh` | 外部 300 并发 curl gate |
| 文档/ADR | `docs/config.md`, `docs/sessions.md`, `docs/proxy-usage.md`, `docs/admin-api.md`, `docs/persistence.md`, `docs/adr/0043`, `0048`, `0049`, `0050`, `0051`, `0061` | 产品边界、性能/可用性接受标准 |

---

## 2. 全链路运行路径

### 2.1 启动链路

证据：`internal/app/app.go:96-147`

```text
config.Load
  -> cfg.Check
  -> logging.New
  -> storage.New / LoadPool
  -> core.NewPool / pool.Replace
  -> core.NewSessionManager
  -> newPluginManager
  -> admin.New / admin.Start
  -> proxy.NewServer / proxy.Start
  -> pluginManager.Start
```

关键行为：

- `LoadPool()` 失败只 degraded log，不阻断启动：`internal/app/app.go:124-127`。
- 持久化候选存在则 `pool.Replace(loaded)`：`internal/app/app.go:128-131`。
- Admin API 先于 proxy listener 启动：`internal/app/app.go:136-145`。
- 插件刷新在监听启动后才开始：`internal/app/app.go:146-147`。

### 2.2 插件刷新/验活/入池链路

证据：`internal/app/app.go:227-280`

```text
plugin.Refresh(ctx)
  -> validation.Validate(ctx, candidates, dialers)
  -> resetIdleDialerCaches(dialers)
  -> pool.AddValidated(valid, dialers)
  -> store.SaveSnapshot(...)
  -> store.SavePool(pool.List())
  -> plugin status update
```

关键行为：

- 无候选或 report error 会使 plugin degraded：`internal/app/app.go:249-267`。
- 每次 refresh 后都会保存 snapshots 和完整 pool：`internal/app/app.go:268-276`。
- sing-box dialer 支持 `ResetIdleCache()` 时会被 reset，并调用 `debug.FreeOSMemory()`：`internal/app/app.go:260-263`, `283-292`。

### 2.3 请求链路

证据：`internal/proxy/server.go:122-269`

```text
Accept TCP
  -> Peek first byte
  -> HTTP path or SOCKS5 path
  -> authenticate / ParseSessionUsername
  -> dialScheduled(target, sessionInfo)
  -> pickCandidateForAttempt
  -> DialViaCandidate or custom sing-box dialer
  -> tunnel or one-shot HTTP forwarding
  -> MarkSuccess / MarkFailure
```

关键行为：

- HTTP CONNECT 和 SOCKS5 都通过 `dialScheduled()` 选择候选：`internal/proxy/server.go:191`, `226`。
- 非 CONNECT HTTP 请求只处理一个 request/response，并强制 `Connection: close`：`internal/proxy/server.go:247-253`。
- 请求级重试只在同一路由过滤条件内进行：`internal/proxy/server.go:276-323`。
- tunnel 使用 2 个 `io.Copy` goroutine 双向拷贝：`internal/proxy/server.go:326-349`。

### 2.4 路由/Session 链路

证据：`internal/core/session.go:36-127`, `129-164`

- legacy：`aio-session-30m`
- structured：`aio~plugin=fpl~region=US~session=job-001~ttl=30m`
- `plugin` 小写、`region` 大写；绑定 key 包含 plugin/region/session：`internal/core/session.go:91-99`, `122-127`。
- 过滤后空集不会 fallback：由 `MatchesRoute()` + `PickMatching()` 实现，测试覆盖 `TestHTTPProxyRouteFailureDoesNotFallbackGlobalPool`。

### 2.5 持久化/Admin 链路

- pool：`data_dir/pool.json`，版本号 `StateVersion=1`：`internal/storage/storage.go:16`, `31-35`, `65-72`。
- snapshots：`data_dir/snapshots/<source>/<timestamp>.json`，保留最近 N 份：`internal/storage/storage.go:75-94`, `109-126`。
- Admin endpoints：`/health`, `/stats`, `/pool`, `/plugins`, `/snapshots`：`internal/admin/admin.go:31-38`。

---

## 3. P0 发现

### P0-001：默认 `random` 调度在大候选池下是 O(N) 分配型热路径，实测会造成灾难性 CPU/GC 压力

**类别**：性能 / 内存分配 / 可用性
**影响面**：所有使用默认 `scheduler.policy=random` 的部署；候选池越大，影响越严重。
**证据文件**：

- `internal/config/config.go:21`：默认调度策略为 `random`。
- `internal/config/config.go:268-270`：未配置时应用默认策略。
- `internal/core/pool.go:145-174`：`PickMatchingExcluding()` 在非 round-robin 路径下，每次请求创建 `available := make([]Candidate, 0, len(p.items))`，遍历整个 `p.order` 并 append 所有可用候选，最后 `rand.Intn`。

**本地实测**：`tmp/2026-06-29-pool-hotpath.log`

```text
pool candidates=10000
allocs/random_pick=1.00
allocs/round_robin_pick=0.00
random_5000 elapsed=19.235987186s total_alloc_delta=12520082KB mallocs_delta=5545
round_robin_5000 elapsed=2.055599ms total_alloc_delta=0KB mallocs_delta=0
```

**解释**：

每次随机 pick 都复制/append 大量 `Candidate` 结构到临时 slice。`Candidate` 内含多个 string、map、time 字段，单次分配很重。10,000 候选下 5,000 次随机 pick 产生约 12.5GB TotalAlloc；这不是压测噪声，而是代码路径必然行为。

**触发条件**：

- 默认 `scheduler.policy=random`；
- FPL/FOFA/sing-box 源积累到数千或更多候选；
- 中等以上请求量；
- additive pool 长期不清理导致候选数增长时会进一步放大。

**用户可感知表现**：

- CPU 飙升；
- GC 频繁；
- 请求延迟明显；
- Admin `/pool` 和持久化也因池大进一步变慢；
- 代理看似“随机不稳定”，实际是调度热路径被候选数拖垮。

**修复建议**：

1. 把 random pick 改成 **无全量分配算法**：
   - 方案 A：在 `order` 上随机起点线性探测，找到第一个可用且 match 的候选；不分配 slice。
   - 方案 B：reservoir sampling，在一次遍历中保留一个随机命中候选；O(N) 但 O(1) 内存。
   - 方案 C：维护按 route/plugin/region 的可用索引，入池/状态变更时更新，pick O(1)/O(logN)。
2. 对 `PickMatchingExcluding` 增加 benchmark 和 allocation gate：
   - 10k 候选 random pick：`allocs/op` 应为 0 或接近 0；
   - 300 并发测试应增加“大池 + random”场景，而不是单候选本地 origin。

**建议验证命令**：

```bash
GOFLAGS=-mod=readonly go test ./internal/core -run Test -bench 'Pick|Pool' -benchmem
GOFLAGS=-mod=readonly go test ./internal/proxy -run TestProxy300Concurrent -count=1 -v
```

---

### P0-002：sing-box `direct` outbound 被当作可用代理候选导入，会绕过“使用上游代理”的产品语义

**类别**：可用性 / 代理语义 / 隐私边界
**影响面**：任何导入 sing-box native config、Clash-like config 或订阅时包含 `direct` outbound 的部署。
**证据文件**：

- `internal/plugins/singbox/singbox.go:689-695`：`convertibleType()` 明确允许 `direct` 和 `block`。
- `internal/plugins/singbox/singbox.go:777-800`：`cleanOutboundMap()` 对 `direct`/`block` 只清理字段，但不拒绝。
- `internal/plugins/singbox/singbox.go:179-193`：`buildSingleOutbound()` 对每个 outbound 创建 `ProtocolSingBox` candidate 和 lazy dialer。
- `docs/adr/0001`/`CONTEXT.md` 的产品词汇强调的是 Proxy Candidate / Upstream Proxy；`direct` 不是上游代理。

**本地 smoke 证据**：`tmp/2026-06-29-direct-outbound-smoke.log`

使用 200 个 sing-box `direct` outbounds 的配置启动服务后，通过 AIOPROXY 访问本地 origin：

```text
### direct outbound proxy smoke
direct-ok
{'count': 200, 'first': {'Protocol': 'singbox', 'Host': '127.0.0.1', 'Source': 'singbox', 'Name': 'direct-0', 'Status': 'available', ...}}
```

**解释**：

`direct` outbound 的行为是直接从 AIOPROXY 所在机器拨目标地址，而不是经第三方上游代理。如果用户导入的订阅里包含 direct 节点，当前系统会把它作为候选入池并调度。对代理聚合器而言，这会造成：

- 出口 IP 变成 AIOPROXY 服务器本机；
- plugin/region routing 对 direct 节点没有真实代理地区意义；
- 误以为“走代理”，实际直连；
- 在封禁、风控、隐私、合规场景下表现为严重可用性/语义错误。

**触发条件**：

- sing-box source 中出现 `type: direct`；
- `validation.skip_validation=true` 时必然可入池；
- 默认 HTTP 验活在服务器本机可访问 validation URL 时也可能通过；
- random/round_robin 都可能调度 direct candidate。

**修复建议**：

1. 默认拒绝 `direct` 和 `block` 作为 proxy candidate：
   - `convertibleType()` 从允许列表移除 `direct`/`block`；
   - import report 记录 `unsupported_direct` / `unsupported_block`。
2. 如果确实需要 direct 用于本地实验，必须加显式配置开关，例如：
   - `plugins.singbox.allow_direct: false` 默认 false；
   - Admin `/pool` 明确标注 direct candidate，health warning。
3. 增加回归测试：
   - `direct` 默认 skip；
   - 开启 allow_direct 时才可导入；
   - `block` 永不作为可调度候选。

**建议验证命令**：

```bash
GOFLAGS=-mod=readonly go test ./internal/plugins/singbox -run 'Direct|Unsupported' -count=1 -v
GOFLAGS=-mod=readonly go run ./cmd/aioproxy check -c examples/config.yaml
```

---

## 4. P1 发现

### P1-001：additive pool 不清理源中已消失候选，长期运行后会造成候选池和 dialer map 膨胀，并放大 P0 random 热路径

**类别**：内存 / 性能 / 可用性
**证据文件**：

- `internal/core/pool.go:39-65`：`AddValidated()` 只新增/更新候选，没有按 source 删除旧候选。
- `internal/core/pool.go:57-60`：有 dialer 时写入 `p.dialers[c.Fingerprint]`，没有删除 disappeared fingerprint 的路径。
- `docs/persistence.md`：明确“刷新是 additive：新验活通过的候选合并进当前池，不覆盖整个池”。
- `internal/storage/storage.go:65-72`：保存的是完整当前 pool。

**影响**：

- 订阅源/FOFA/FPL 的候选天然高 churn；源中消失的候选会继续留在池中，直到运行时失败达到阈值才 unavailable。
- 如果某些候选长期不被调度，它们不会自然消失。
- sing-box lazy dialer 即使 idle 关闭 box，也仍保留每个旧节点的 per-node JSON config、metadata、timer 状态对象；`p.dialers` 不清理会形成长期内存保留。
- pool 越大，P0 random pick、Admin `/pool`、SavePool、snapshot JSON 都越慢。

**触发条件**：

- 运行数天/数周；
- 上游源频繁变动；
- FPL/FOFA/sing-box 都启用；
- 默认 additive 策略叠加 random 调度。

**修复建议**：

1. 在 refresh 维度增加 **source-aware reconciliation**：
   - 记录候选 `Source` + source label / query；
   - 对本次刷新来源中未出现的旧候选标记 stale 或 unavailable；
   - 可配置 grace period 后删除。
2. `Pool` 增加删除 API，必须同步删除 `items/order/dialers`。
3. 为 additive 策略保留兼容路径，但增加最大池大小、stale TTL、last_seen 字段。
4. 增加长跑测试：模拟每轮 1,000 个不同 sing-box 节点刷新 20 轮，断言 pool/dialers 不无限增长。

---

### P1-002：sing-box 验证阶段仍然存在与候选数 × 并发相关的瞬时高 RSS/CPU 峰值

**类别**：内存峰值 / CPU / 可用性
**证据文件**：

- `internal/validation/validation.go:25-70`：按 `cfg.Concurrency` 并发验证候选。
- `internal/validation/validation.go:78-160`：每个候选通过 `DialViaCandidate()` 完整发起代理验活请求。
- `internal/plugins/singbox/singbox.go:230-240`：lazy dialer 的每次首次 dial 会 `acquire()`。
- `internal/plugins/singbox/singbox.go:259-291`：`ensureLocked()`/`startOutboundBox()` 会创建并启动一个 sing-box `box.Box`。
- `internal/app/app.go:260-263`：验证后尝试 reset idle cache 并 `debug.FreeOSMemory()`，说明代码已经在处理验证峰值后的释放。

**当前正向证据**：

200 个 sing-box direct outbound、skip validation 的 idle smoke：`tmp/2026-06-29-mem-smoke.log`

```text
plugin refresh finished plugin=singbox imported=200 validated=200 degraded=false
VmHWM: 26448 kB
VmRSS: 25360 kB
Threads: 14
{"pool_available":200,"pool_total":200,"status":"healthy"}
```

这说明当前 lazy 改造已经解决“导入即常驻 200 个 box”的旧问题。

**仍存在的问题**：

在非 skip validation、尤其 `ip_api_country` 场景，每个 sing-box candidate 需要真实经该节点访问验证 URL。由于 lazy dialer 首次拨号会创建 `box.Box`，刷新窗口内仍会同时存在多个 sing-box box，峰值由：

```text
min(validation.concurrency, candidates) × 单 box 启动成本
```

决定。默认 `validation.concurrency=100`，对低内存 VPS 很危险。

**修复建议**：

1. 增加 sing-box 专用验证并发上限，例如：
   - `plugins.singbox.validation_concurrency`；
   - 默认远低于全局 100，例如 5-10。
2. validation worker 数改为 `min(concurrency, len(candidates))`。
3. 对 sing-box 验证增加分批与 backpressure；刷新状态报告中记录 peak/inflight。
4. 增加运行态测试：500/1000 sing-box candidates + ip-api local mock，记录 VmHWM/VmRSS，作为回归 gate。

---

### P1-003：tunnel 的 early zero-byte heuristic 会误伤空闲但仍打开的 CONNECT/SOCKS 隧道

**类别**：可用性 / 误淘汰
**证据文件**：`internal/proxy/server.go:326-349`

当前逻辑：

1. 启动两个 `io.Copy` goroutine；
2. `select` 等待任一方向结束或 `EarlyFailureWindow` 到期；
3. 如果窗口内累计字节数为 0，则 `MarkFailure(cand, "early zero-byte closure")`；
4. 之后 `<-done` 等待一个 copy 结束。

问题点：

- 如果客户端建立 CONNECT 后暂时不发送数据，或 SOCKS 隧道建立后应用层慢启动，窗口到期时 bytes 仍为 0，候选会被记失败。
- 这不是“early zero-byte closure”，因为连接可能并未 closure，只是 idle。
- 误计失败累计到 `max_failures` 后，该候选会被标记 unavailable。

**配置边界叠加问题**：

`runtime_failure.early_failure_window` 没有在 `Config.Check()` 中校验正数。实际验证：`tmp/2026-06-29-bad-timing-check.log`

```text
AIOPROXY config check
active plugins: fpl
result: ok
```

该配置中包含：

```yaml
runtime_failure:
  early_failure_window: "-1s"
refresh:
  jitter_ratio: -2
```

但 `check` 仍通过。

**修复建议**：

1. 把 heuristic 改成只在“窗口内连接已经结束且 0 字节”时计失败；不要在仍打开但 idle 时计失败。
2. 或区分：
   - `early_zero_byte_close_window`：只对已关闭连接生效；
   - `idle_timeout`：单独作为连接 idle 管理策略，不直接等同代理失败。
3. `Config.Check()` 增加：
   - `early_failure_window > 0`；
   - 或允许 0 表示禁用，但语义必须明确。
4. 增加测试：建立 CONNECT 后 idle 超过窗口再发送数据，不应导致候选 failure_count 增加。

---

### P1-004：代理入口没有读超时、头大小上限或连接保护；可信小规模边界外会被慢连接拖住 goroutine/内存

**类别**：可用性 / 资源保护
**证据文件**：

- `internal/proxy/server.go:41-50`：原始 TCP listener，无 `http.Server` 风格超时。
- `internal/proxy/server.go:102-119`：每个 accepted conn 启动 goroutine。
- `internal/proxy/server.go:122-134`：`bufio.NewReader(conn)` 后 `Peek(1)` 无 deadline。
- `internal/proxy/server.go:136-199`：SOCKS 握手读无 deadline。
- `internal/proxy/server.go:201-220`：`http.ReadRequest(br)` 无 read deadline/max header bound。
- `docs/adr/0049-no-connection-limits-v1.md`：v1 明确不做连接限制。

**影响**：

在“单实例自用/小可信客户端集”的 ADR 边界内，这不是立即阻断；但只要代理端口暴露给 LAN 或公网，慢连接可长期占用 goroutine、conn map、文件描述符和少量 heap。300 并发测试并不能覆盖慢连接攻击或异常客户端。

**修复建议**：

1. 至少增加握手阶段 deadline：
   - accept 后设置 `SetReadDeadline(now + handshakeTimeout)`；
   - 认证和首个请求完成后清除或改为 idle deadline。
2. HTTP path 使用 `http.MaxBytesReader` 不适用于裸 conn，但可以改用 `textproto.Reader` 限制 header，或用 `http.Server` 处理 HTTP proxy path。
3. SOCKS username/password、domain length 已天然 1 byte 限制，但整体握手仍需 deadline。
4. 文档明确：若暴露到非 loopback，必须放在防火墙/内网可信范围内；v1 不提供抗滥用保障。

---

### P1-005：refresh jitter 缺少边界校验，异常值可造成刷新风暴或非预期长间隔

**类别**：可用性 / 上游负载 / 配置安全
**证据文件**：

- `internal/app/app.go:230-235`：`jitter := (rand.Float64()*2 - 1) * ratio * interval`，然后 `time.NewTimer(interval + jitter)`。
- `internal/config/config.go:330-332`：默认 `jitter_ratio=0.1`。
- `internal/config/config.go:375-458`：`Check()` 没有校验 `Refresh.JitterRatio` 范围。

**实际验证**：`tmp/2026-06-29-bad-timing-check.log`

```yaml
refresh: { jitter_ratio: -2 }
```

仍然：

```text
result: ok
```

**影响**：

- `ratio > 1` 时 `interval + jitter` 可能为负或接近 0，timer 立即触发，形成刷新风暴。
- `ratio < 0` 时行为反直觉，同样可能扩大随机区间。
- 源请求、验证并发、SavePool、snapshots 会被刷新风暴串联放大。

**修复建议**：

- `Config.Check()` 强制 `0 <= jitter_ratio <= 1`，建议进一步约束到 `<=0.5`；
- timer duration 小于最小刷新间隔时 clamp，例如不低于 `interval * 0.1` 或固定 `1m`；
- 增加配置测试覆盖 negative 和 >1。

---

### P1-006：graceful shutdown 对正在 dial 的请求没有统一取消链路，最坏会超过 grace 继续残留 goroutine

**类别**：可用性 / 生命周期
**证据文件**：

- `internal/proxy/server.go:276-294`：`dialScheduled()` 使用 `context.WithTimeout(context.Background(), dialAttemptTimeout)`，不是 server/shutdown ctx。
- `internal/proxy/server.go:87-99`：`Wait()` 超时后强制关闭 active client conns，再最多等 2s。
- `internal/app/app.go:150-154`：shutdown 顺序为 `proxySrv.Close()` -> `pm.Stop()` -> `proxySrv.Wait()`。

**影响**：

- shutdown 关闭 listener 和 client conns，但不会取消已经进入 `DialViaCandidate()` 的上游 dial context。
- 如果上游 dial 卡住或 custom dialer 不响应 client conn close，handler goroutine 可能持续到固定 30s attempt timeout，超过 configured grace period。
- `pm.Stop()` 在 `proxySrv.Wait()` 前取消 plugin ctx，可能影响仍在使用 sing-box lazy dialer 的活动连接；虽然进程最终退出，但 graceful 语义不够干净。

**修复建议**：

1. `Server` 持有 lifecycle ctx；`Close()` cancel 它。
2. `dialScheduled()` 用 `context.WithTimeout(serverCtx, dialAttemptTimeout)`。
3. shutdown 顺序建议：
   - close listener；
   - 等活动 proxy 连接 grace；
   - 再 stop plugin manager / close dialers；
   - save pool；
   - shutdown admin。
4. 增加测试：custom dialer 阻塞，调用 `Close/Wait(短 grace)` 后 goroutine 不残留或可观测退出。

---

## 5. P2 发现

### P2-001：`Validator.Validate()` worker 数不按候选数收敛，且缺少配置上限

**类别**：性能 / 内存 / goroutine 管理
**证据文件**：`internal/validation/validation.go:25-70`

当前行为：

- `workers := cfg.Concurrency`；
- 即使候选只有 1 个，也会启动 `workers` 个 goroutine；
- `out := make(chan Candidate, len(candidates))` 对大候选数开全量 buffer。

**影响**：

- 默认 100 worker 对普通场景可接受，但配置成数千会被 `check` 接受；
- 对 sing-box 候选会放大 P1-002 峰值；
- 对小候选数是无意义 goroutine 开销。

**建议**：

```go
workers = min(max(1, cfg.Concurrency), len(candidates), globalMaxValidationWorkers)
```

并在 `Config.Check()` 中给出硬上限或 warning。

---

### P2-002：持久化每次刷新全量 `MarshalIndent` 写 pool 和 candidate snapshots，池大后 I/O 与 heap 峰值明显

**类别**：性能 / 内存 / 磁盘
**证据文件**：

- `internal/app/app.go:268-276`：每次 refresh 后保存 snapshot 和完整 pool。
- `internal/storage/storage.go:65-72`：`SavePool()` 写完整候选数组。
- `internal/storage/storage.go:75-94`：`SaveSnapshot()` 写 report + candidates。
- `internal/storage/storage.go:128-162`：`json.MarshalIndent()` 先完整构造 `[]byte`，然后写临时文件、fsync、rename。

**影响**：

- 10k/100k 候选时，每次刷新都会产生大型 JSON buffer；
- `MarshalIndent` 比紧凑 JSON 更占 CPU/磁盘；
- SaveSnapshot 与 SavePool 串行持有 Store mutex，会阻塞 Admin `/snapshots` 等存储操作；
- snapshots 包含候选数组，磁盘增长与候选数、来源数、retention 成正比。

**建议**：

- pool 持久化使用 streaming encoder 或紧凑 JSON；
- snapshot 可只保存 report + candidate IDs/统计，完整候选另存可选；
- 对大池增加 SavePool 节流：只有 changed/interval/shutdown 时写；
- 在 report 中暴露 pool file size、snapshot count 作为 health metadata。

---

### P2-003：Admin `/pool` 和 `/snapshots` 全量返回，缺少分页/过滤，池大后会产生响应延迟和内存峰值

**类别**：可用性 / 管理面性能
**证据文件**：

- `internal/admin/admin.go:91-101`：`/pool` 遍历 `pool.List()` 并构造完整 view slice。
- `internal/core/pool.go:80-90`：`List()` 复制完整候选 slice。
- `internal/admin/admin.go:104-107`：`/snapshots` 返回完整文件列表。
- `internal/storage/storage.go:96-107`：`SnapshotFiles()` walk 全部 snapshot tree。

**影响**：

- 大池下一次 `/pool` 至少复制两次数组：`pool.List()` + admin view；
- JSON encode 可能阻塞管理面；
- `/snapshots` 随历史文件数增长。

**建议**：

- `/pool?limit=&offset=&source=&status=&plugin=&region=`；
- `/stats` 暴露聚合计数，不让 UI/运维默认拉全量；
- `/snapshots` 分页或只按 source 返回最近 N 个。

---

### P2-004：sing-box source 解析存在多重内容复制，file/inline 缺少大小限制

**类别**：内存峰值 / 输入健壮性
**证据文件**：

- `internal/plugins/singbox/singbox.go:128-150`：URL source 用 `LimitReader(32<<20)`，file source `os.ReadFile` 无限制，inline 直接取 `src.URL`。
- `internal/plugins/singbox/singbox.go:383-413`：`strings.TrimSpace(string(content))` 复制为 string；base64 decode 再复制；YAML unmarshal 到 map 再复制结构。

**影响**：

- 大 file source 可一次性读入内存；
- URL source 超过 32MiB 时被截断但没有明确报错，后续解析错误不指向“超限”；
- YAML/native config 会在 content bytes、string、map、jsonBytes per outbound 间产生多份数据。

**建议**：

- file/inline 也统一限制大小；
- URL `LimitReader` 读 `limit+1`，超过时报 `source too large`；
- 对 share-link list 走 streaming scanner，避免整文件 string 化；
- 对 YAML/native config 保留硬限制并报告 node count。

---

### P2-005：FPL parser 未检查 `Scanner.Err()`，长行或读错误会被静默吞掉

**类别**：可用性 / 可观测性
**证据文件**：`internal/plugins/fpl/fpl.go:63-97`

当前 scanner 设置：

```go
s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
for s.Scan() { ... }
return out
```

没有在循环后检查 `s.Err()`。超过 1MiB 的行、底层 reader 错误等都会导致 scanner 停止，但 report 不会记录 error 或 skip reason。

**建议**：

- 循环后：
  - `if err := s.Err(); err != nil { report.Error = ... }`；
  - 对超长行记录 `line_too_long`。
- 增加测试：输入 >1MiB 单行，应有 report error。

---

### P2-006：FOFA 查询串行且响应体无大小上限，异常 API 或代理返回可放大内存/延迟

**类别**：性能 / 内存 / 可用性
**证据文件**：

- `internal/plugins/fofa/fofa.go:37-52`：queries 串行执行。
- `internal/plugins/fofa/fofa.go:75-86`：`json.NewDecoder(resp.Body).Decode(&sr)` 无 body limit。
- `internal/config/config.go:345-347`：默认 size=100，但没有校验上限。

**影响**：

- 多 query 时刷新总耗时相加；
- `size` 配置过大或 API 异常返回超大 JSON 时，会直接解码到 `[][]any`；
- key 放在 query string 中，虽然当前不主动记录 URL，但代理/中间件日志可能记录完整 URL。

**建议**：

- `io.LimitReader` 包住 response body；
- `Config.Check()` 对 `fofa.size` 设置合理上限；
- 可选并发 query，但要受 rate limit 控制；
- 避免在错误中输出完整请求 URL。

---

### P2-007：HTTP 非 CONNECT 路径强制短连接，吞吐与延迟受影响

**类别**：性能 / 用户体验
**证据文件**：`internal/proxy/server.go:247-253`

当前：

```go
req.Close = true
req.Header.Set("Connection", "close")
```

每个 HTTP 请求都新建上游代理连接，无法复用 client-side 或 upstream-side keep-alive。对大量短 HTTP 请求，成本明显高于连接复用。

**建议**：

- 保留 v1 简洁性时可只文档说明；
- 若要优化，考虑 HTTP proxy path 使用 `http.Transport` 风格连接池，或明确只优化 CONNECT/SOCKS 长连接场景。

---

### P2-008：测试 gate 覆盖了功能与本地 300 并发，但没有覆盖“大池 + random + sing-box 验证峰值 + 慢连接”

**类别**：验证完整性
**证据文件**：

- `internal/proxy/server_test.go:63-113`：`TestProxy300Concurrent` 只有 1 个 candidate、本地 origin、DirectDialer。
- `scripts/stress_300.sh:1-6`：外部 curl 300 并发脚本，但不构造大池/慢连接/高 churn 源。

**影响**：

现有 gate 能证明基础并发不崩，但不能证明：

- 默认 random 在 10k 候选下可用；
- sing-box 验证峰值可接受；
- 慢连接不会拖垮 goroutine；
- additive pool 长跑不膨胀。

**建议新增 gate**：

1. `TestPoolRandomPickLargeNoAllocation`；
2. `TestProxyLargePool300ConcurrentRandom`；
3. `TestValidationSingBoxConcurrencyCap`；
4. `TestSlowHandshakeTimeout`；
5. `TestRefreshReconcileRemovesStaleDialers`。

---

## 6. P3 发现

### P3-001：Admin token 比较不是 constant-time

**类别**：安全卫生 / 管理面
**证据文件**：`internal/admin/admin.go:60-70`

当前直接字符串比较：

```go
token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
if token != s.cfg.Token { ... }
```

在默认 loopback 管理面下影响很低；若 Admin 暴露到网络，可改为 `subtle.ConstantTimeCompare`。

---

### P3-002：Admin `/plugins` 可能输出较多 import report metadata，长期需要继续审视敏感字段

**类别**：信息暴露卫生
**证据文件**：

- `internal/admin/admin.go:103`：直接输出 `s.statuses()`。
- `internal/core/types.go:102-114`：ImportReport 可带 `Metadata`。
- `docs/admin-api.md` 声称不返回 proxy password、FOFA key、完整订阅 URL 或 raw node。

当前 FPL `sourceLabel()` 已避免 URL credential 泄漏，且测试覆盖：`internal/plugins/fpl/fpl_test.go:23-33`。但后续新增插件/metadata 时需要保持 Admin 输出白名单策略。

---

### P3-003：`configPath()` 手写参数解析，不支持未知参数提示和重复参数处理

**类别**：CLI 可用性
**证据文件**：`internal/app/app.go:53-63`

当前足够支撑 v1，但后续 CLI 扩展时建议使用 `flag.FlagSet`，避免 `aioproxy serve -c a -x` 静默忽略未知参数。

---

## 7. 维度归纳

### 7.1 性能问题清单

| 优先级 | 问题 | 核心证据 | 主要修复方向 |
|---|---|---|---|
| P0 | random pick 每请求全量分配候选 slice | `pool.go:145-174`；10k/5k pick = 12.5GB TotalAlloc | O(1) 内存 random pick / route index |
| P1 | additive pool 长期增长 | `pool.go:39-65`；`docs/persistence.md` | source-aware reconcile / stale TTL |
| P1 | refresh jitter 异常值导致刷新风暴 | `app.go:230-235`；bad timing check 通过 | 校验 ratio 范围和最小 timer |
| P2 | Admin `/pool` 全量复制/编码 | `admin.go:91-101` | 分页/过滤 |
| P2 | SavePool/Snapshot 全量 MarshalIndent | `storage.go:128-162` | streaming/紧凑 JSON/节流 |
| P2 | 非 CONNECT HTTP 强制短连接 | `server.go:247-253` | 可选连接复用 |

### 7.2 内存占用问题清单

| 优先级 | 问题 | 核心证据 | 主要修复方向 |
|---|---|---|---|
| P0 | random pick 巨量临时分配 | 微基准 12.5GB TotalAlloc | 无分配 pick |
| P1 | stale dialers/config bytes 长期保留 | `pool.go:57-60` 无删除 | 删除 API + reconcile |
| P1 | sing-box validation 峰值仍高 | `validation.go` + `singbox.go` start box | sing-box 专用并发 cap |
| P2 | storage JSON 全量构造 | `json.MarshalIndent` | streaming encoder |
| P2 | sing-box parse 多份复制/file 无限制 | `singbox.go:128-150`, `383-413` | size limit + streaming |
| P2 | FOFA decode 无 body limit | `fofa.go:84-86` | response body cap |

### 7.3 可用性问题清单

| 优先级 | 问题 | 核心证据 | 主要修复方向 |
|---|---|---|---|
| P0 | sing-box direct 被调度，绕过上游代理语义 | `convertibleType()` 允许 direct；smoke 成功 | 默认拒绝 direct/block |
| P1 | idle tunnel 被早期零字节误淘汰 | `server.go:326-349` | 只对已关闭 0 字节连接计失败 |
| P1 | proxy listener 无读超时，慢连接可耗资源 | `server.go:122-220` | handshake/read deadline |
| P1 | shutdown dial context 不受 server cancel 控制 | `server.go:276-294` | server lifecycle ctx |
| P1 | jitter/early window 配置非法仍 check ok | `config.go:375-458` | config validation |
| P2 | FPL Scanner.Err 静默 | `fpl.go:63-97` | report error |

---

## 8. 建议修复路线

### 第一批：必须优先修复（P0）

1. **重写 random pick**：
   - 目标：大池 pick 0 allocation；
   - 增加 benchmark 和 regression test；
   - 保证 route/exclude/session retry 语义不变。

2. **禁止 sing-box direct/block 默认入池**：
   - `convertibleType()` 默认不允许 direct/block；
   - report skip reason；
   - 如需实验开关，必须显式配置并 warning。

### 第二批：稳定性与内存治理（P1）

3. **source-aware pool reconcile**：
   - 新增 last_seen/source_key；
   - 清理 stale items 和 dialers；
   - 保存前确保 pool 不无限增长。

4. **sing-box 验证并发 cap**：
   - `min(global, singbox cap, len(candidates))`；
   - 加 VmHWM 验证脚本。

5. **修正 tunnel early failure**：
   - idle 不等于 failure；
   - 只对已经 closed 的 0 字节连接计失败。

6. **配置边界校验**：
   - `0 <= jitter_ratio <= 1`；
   - `early_failure_window > 0` 或 0 明确表示禁用；
   - `validation.concurrency`/`fofa.size` 上限。

7. **代理握手 deadline**：
   - 防慢连接；
   - 不违背 ADR 0049 的“无产品级连接限额”，只是基础资源保护。

### 第三批：规模化优化（P2/P3）

8. Admin `/pool` `/snapshots` 分页；
9. 持久化 JSON streaming/节流；
10. sing-box/FPL/FOFA 输入 size limit 和错误报告；
11. 非 CONNECT HTTP 复用策略调研；
12. Admin token constant-time compare。

---

## 9. 建议新增验收矩阵

| Gate | 目标 | 示例命令/方式 |
|---|---|---|
| Core tests | 功能不回退 | `GOFLAGS=-mod=readonly go test ./... -count=1` |
| Race tests | 并发无数据竞争 | `GOFLAGS=-mod=readonly go test -race ./... -count=1` |
| Config check | 示例配置仍通过 | `go run ./cmd/aioproxy check -c examples/config.yaml` |
| Existing 300 | 保留现有 gate | `go test ./internal/proxy -run TestProxy300Concurrent -count=1 -v` |
| Large random pick | 10k pool pick 无分配 | `go test ./internal/core -bench Pick -benchmem` |
| Large proxy random | 10k pool + 300 并发 | 新增 proxy integration test |
| Sing-box validation peak | 500/1000 节点 VmHWM 不超阈值 | local ip-api mock + `/proc/$pid/status` |
| Pool reconcile | 20 轮 churn 后 pool/dialers 有界 | 新增 core/app test |
| Idle CONNECT | idle 超 early window 不误淘汰 | 新增 proxy test |
| Slow handshake | 慢连接按 deadline 释放 | 新增 proxy test |

---

## 10. 当前已有测试覆盖与不足

### 已覆盖

- 配置基础校验：`internal/config/config_test.go`
- session username/route/session rebind：`internal/core/session_test.go`
- HTTP/SOCKS5 混合入口：`internal/proxy/server_test.go:24-61`
- 300 并发本地 gate：`internal/proxy/server_test.go:63-113`
- plugin/region route：`internal/proxy/server_test.go:115-159`
- 同路由 retry 和 session rebind：`internal/proxy/server_test.go:161-245`
- route fail-closed：`internal/proxy/server_test.go:247-292`
- storage 并发保存和 snapshot retention：`internal/storage/storage_test.go`
- validation ctx cancel / ip-api country：`internal/validation/validation_test.go`
- sing-box parser/lazy dialer/direct start reset：`internal/plugins/singbox/singbox_test.go`

### 不足

- 没有大池 random allocation/perf gate；
- 没有 pool/dialer stale cleanup 长跑测试；
- 没有真实 sing-box validation peak gate；
- 没有 slowloris/handshake timeout 测试；
- 没有 idle CONNECT 误淘汰测试；
- 没有 Admin `/pool` 大池响应测试；
- 没有 FOFA/FPL/sing-box oversized input 测试。

---

## 11. 附录：关键证据命令

### 11.1 完整验证命令

```bash
GOFLAGS=-mod=readonly go test ./... -count=1
GOFLAGS=-mod=readonly go test -race ./... -count=1
GOFLAGS=-mod=readonly go run ./cmd/aioproxy check -c examples/config.yaml
GOFLAGS=-mod=readonly go test ./internal/proxy -run TestProxy300Concurrent -count=1 -v
GOFLAGS=-mod=readonly go vet ./...
```

### 11.2 大池 random 热路径微基准

```bash
GOFLAGS=-mod=readonly go run tmp/audit_pool_hotpath.go
```

输出：

```text
pool candidates=10000
allocs/random_pick=1.00
allocs/round_robin_pick=0.00
random_5000 elapsed=19.235987186s total_alloc_delta=12520082KB mallocs_delta=5545
round_robin_5000 elapsed=2.055599ms total_alloc_delta=0KB mallocs_delta=0
```

### 11.3 当前 200 sing-box lazy direct idle memory smoke

```bash
GOFLAGS=-mod=readonly go build -o tmp/aioproxy-audit ./cmd/aioproxy
./tmp/aioproxy-audit serve -c tmp/audit-mem-config.yaml
ps -p "$pid" -o pid,ppid,rss,vsz,pmem,comm,args
cat /proc/$pid/status | egrep 'Name|VmRSS|VmHWM|VmData|RssAnon|RssFile|Threads'
curl -fsS http://127.0.0.1:19481/health
```

输出摘录：

```text
RSS=25360 kB
VmHWM=26448 kB
VmRSS=25360 kB
RssAnon=12680 kB
Threads=14
plugin refresh finished plugin=singbox imported=200 validated=200 degraded=false
{"pool_available":200,"pool_total":200,"sessions":0,"status":"healthy"}
```

### 11.4 direct outbound 调度 smoke

```bash
curl -fsS -x http://aio:change-me@127.0.0.1:19480 http://127.0.0.1:19491/
curl -fsS http://127.0.0.1:19481/pool
```

输出摘录：

```text
direct-ok
{'count': 200, 'first': {'Protocol': 'singbox', 'Source': 'singbox', 'Name': 'direct-0', 'Status': 'available'}}
```

### 11.5 非法 timing 配置仍通过 check

```bash
GOFLAGS=-mod=readonly go run ./cmd/aioproxy check -c tmp/audit-bad-timing.yaml
```

输出：

```text
AIOPROXY config check
active plugins: fpl
result: ok
```

---

## 12. 最终审计结论

AIOPROXY 当前不是“缺少完整链路”，而是已经形成了完整的代理聚合器链路；主要风险来自规模化和边界语义：

1. **规模化热路径未优化**：默认 random 调度在大池场景有明确灾难性分配问题，是首要性能修复点。
2. **候选池生命周期不闭环**：additive 策略没有 stale/dialer 清理，长期运行会持续放大调度、存储、Admin、内存问题。
3. **sing-box 已完成 idle 常驻内存优化，但验证峰值和 direct 语义仍需治理**：lazy dialer 已经把 200 节点 idle RSS 控制在约 25MB，但 validation peak 和 direct outbound 入池仍是高优先级风险。
4. **可用性边界依赖“可信小规模客户端”假设**：无读 deadline、无连接保护、early zero-byte heuristic、shutdown cancel 不统一，在暴露范围扩大或异常客户端出现时会变成真实稳定性问题。
5. **现有测试证明基础功能，不证明大池和长期运行稳定**：需要把大池 random、pool reconcile、sing-box peak、慢连接、idle tunnel 纳入验收矩阵。

建议立即进入修复台账的顺序：

```text
P0-001 random pick 无分配化
P0-002 direct/block 默认禁止入池
P1-001 source-aware stale/dialer 清理
P1-002 sing-box validation concurrency cap
P1-003 tunnel early failure 语义修正
P1-005 timing 配置边界校验
P1-004 proxy 握手/read deadline
```

---

## 13. 修复闭环记录（2026-06-29）

本节记录按本报告台账执行的逐项修复、对抗性测试与验收结果。修复后的当前工作树已通过全量测试、race、配置检查、300 并发 gate、P0 benchmark、坏配置拒绝和 `git diff --check`。

### 13.1 P0 修复

| ID | 状态 | 修复内容 | 对抗/回归测试 |
|---|---|---|---|
| P0-001 | fixed | `internal/core/pool.go` 的 random pick 从“每次构造完整 `available []Candidate`”改为随机起点线性探测；默认/密集池不再全池扫描和分配。 | `TestRandomPickLargePoolAvoidsPerPickAllocation`、`TestRandomPickHonorsMatchAndExclude`、`BenchmarkPoolRandomPickLarge`。 |
| P0-002 | fixed | `internal/plugins/singbox/singbox.go` 默认不再把 `direct` / `block` 作为 convertible outbound；导入阶段记录 `unsupported_direct` / `unsupported_block`。lazy dialer 测试改用真实 HTTP CONNECT outbound。 | `TestDirectAndBlockAreSkippedBeforeBuild`、`TestLazySingBoxDialerCanResetAndRecreate`。 |

P0 benchmark 验收摘录：

```text
BenchmarkPoolRandomPickLarge-24  4069992  288.8 ns/op  0 B/op  0 allocs/op
BenchmarkPoolRandomPickLarge-24  4128470  264.4 ns/op  0 B/op  0 allocs/op
BenchmarkPoolRandomPickLarge-24  4765147  260.4 ns/op  0 B/op  0 allocs/op
```

### 13.2 P1 修复

| ID | 状态 | 修复内容 | 对抗/回归测试 |
|---|---|---|---|
| P1-001 | fixed | 新增 `Pool.ReplaceValidatedMatching`，插件刷新按 report source 替换本来源候选，删除 stale candidate/dialer，并对支持 `ResetIdleCache()` 的 stale dialer 先清理 idle 资源；FPL 候选补 `metadata.source`。 | `TestReplaceValidatedMatchingRemovesStaleCandidatesAndDialers`、`TestRefreshAnnotatesCandidatesWithSourceLabel`。 |
| P1-002 | fixed | `plugins.singbox.validation_concurrency` 新增默认值 `10`；sing-box 刷新验活实际使用 `min(validation.concurrency, plugins.singbox.validation_concurrency)`。 | `TestSingBoxRefreshUsesPluginValidationConcurrencyCap`、`TestSingBoxValidationConcurrencyDefaultAndValidation`。 |
| P1-003 | fixed | `tunnel` 只在 early window 内连接已经关闭且 0 字节时计 `early zero-byte closure`；idle 但仍打开的隧道不再被误淘汰。 | `TestTunnelIdlePastEarlyWindowDoesNotMarkFailure`、`TestTunnelEarlyZeroByteCloseMarksFailure`。 |
| P1-004 | fixed | 新增 `server.handshake_timeout` 默认 `5s`；代理入口初始 HTTP/SOCKS5 握手阶段设置 read deadline，握手完成后清除。 | `TestSlowInitialHandshakeTimesOut`、`TestCheckRejectsUnsafeTimingAndSizeBounds/negative_handshake_timeout`。 |
| P1-005 | fixed | `Config.Check()` 增加 `refresh.jitter_ratio`、`runtime_failure.early_failure_window`、`validation.concurrency`、`plugins.fofa.size` 边界校验。 | `TestCheckRejectsUnsafeTimingAndSizeBounds`；`tmp/audit-bad-timing.yaml` 现在返回非 0。 |
| P1-006 | fixed | `proxy.Server` 持有 lifecycle context；`Close()` 取消 context；`dialScheduled()` 的 30s dial timeout 从 server context 派生，shutdown 可取消 in-flight dial。 | `TestCloseCancelsInFlightDial`。 |

坏配置拒绝验收摘录：

```text
AIOPROXY config check
active plugins: fpl
ERROR: runtime_failure.early_failure_window must be positive
ERROR: refresh.jitter_ratio must be between 0 and 1
exit status 1
bad_timing_exit=1
```

### 13.3 P2 修复

| ID | 状态 | 修复内容 | 对抗/回归测试 |
|---|---|---|---|
| P2-001 | fixed | `Validator.Validate()` worker 数收敛到 `min(concurrency, len(candidates))`；配置层限制 `validation.concurrency <= 1000`。 | `TestValidateStopsDispatchWhenContextCanceled`、`TestCheckRejectsUnsafeTimingAndSizeBounds/validation_concurrency_too_high`。 |
| P2-002 | fixed | `storage.writeJSONAtomic()` 从 `json.MarshalIndent` 全量构造 buffer 改为 `json.Encoder` streaming 写入临时文件。 | `TestConcurrentSavePool`、`TestConcurrentSaveSnapshotRetention`。 |
| P2-003 | fixed | Admin `/pool` 支持 `limit`、`offset`、`source`、`status`、`protocol`；默认 limit=1000，最大 5000；响应头返回 matched/returned/limit/offset；实现避免构造全量 view slice。 | `TestPoolViewSupportsPaginationAndFilters`。 |
| P2-004 | fixed | sing-box URL/file/inline source 统一走 size limit，超过 `maxSingBoxSourceBytes` 返回 `source too large`，不再静默截断 URL source。 | `TestReadSourceRejectsOversizedInline`。 |
| P2-005 | fixed | FPL parser 在 scanner loop 后检查 `Scanner.Err()` 并写入 report error。 | `TestParseReportsScannerError`。 |
| P2-006 | fixed | FOFA response body 增加硬上限，超限返回 report error；配置层限制 `plugins.fofa.size <= 10000`。 | `TestSearchRejectsOversizedResponse`、`TestCheckRejectsUnsafeTimingAndSizeBounds/fofa_size_too_high`。 |
| P2-007 | fixed | HTTP proxy 非 CONNECT 路径支持同一客户端连接上的连续请求；仍保持每个上游请求独立调度和上游 `Connection: close` 简洁语义。 | `TestHTTPProxyKeepsClientConnectionForSequentialRequests`。 |
| P2-008 | fixed | 新增大池 random allocation benchmark、direct/block skip、sing-box validation cap、idle tunnel、slow handshake、shutdown dial cancel、Admin pagination、oversized input 等 gates，补齐原报告指出的验证盲区。 | 全量 `go test ./...`、`go test -race ./...`、专项 tests/benchmark。 |

### 13.4 P3 修复

| ID | 状态 | 修复内容 | 对抗/回归测试 |
|---|---|---|---|
| P3-001 | fixed | Admin bearer token 比较改为 `crypto/subtle.ConstantTimeCompare`。 | `go test ./internal/admin`。 |
| P3-002 | fixed | Admin `/plugins` 输出前对 `ImportReport.Metadata` 做白名单过滤，仅保留 `type` / `protocol`，避免未来插件把 raw URL/token 等 metadata 透出。 | `TestPluginsViewScrubsReportMetadata`。 |
| P3-003 | fixed | CLI `-c/--config` 解析改为 `flag.FlagSet`；未知参数、缺失值、多余 positional arg 不再静默忽略。 | `TestConfigPathRejectsUnknownOrExtraArgs`。 |

### 13.5 最终验收命令

```bash
GOFLAGS=-mod=readonly go test ./... -count=1
GOFLAGS=-mod=readonly go test -race ./... -count=1
GOFLAGS=-mod=readonly go run ./cmd/aioproxy check -c examples/config.yaml
GOFLAGS=-mod=readonly go test ./internal/proxy -run TestProxy300Concurrent -count=1 -v
GOFLAGS=-mod=readonly go test ./internal/core -run '^$' -bench BenchmarkPoolRandomPickLarge -benchmem -count=3
GOFLAGS=-mod=readonly go run ./cmd/aioproxy check -c tmp/audit-bad-timing.yaml  # expected non-zero
GOFLAGS=-mod=readonly go vet ./...
git diff --check
```

最终验收输出保存在：

```text
tmp/2026-06-29-ledger-fix-verification.log
```

关键通过输出：

```text
ok   github.com/aioproxy/aioproxy/internal/admin
ok   github.com/aioproxy/aioproxy/internal/app
ok   github.com/aioproxy/aioproxy/internal/config
ok   github.com/aioproxy/aioproxy/internal/core
ok   github.com/aioproxy/aioproxy/internal/plugins/fofa
ok   github.com/aioproxy/aioproxy/internal/plugins/fpl
ok   github.com/aioproxy/aioproxy/internal/plugins/singbox
ok   github.com/aioproxy/aioproxy/internal/proxy
ok   github.com/aioproxy/aioproxy/internal/storage
ok   github.com/aioproxy/aioproxy/internal/validation
--- PASS: TestProxy300Concurrent
0 B/op  0 allocs/op
bad_timing_exit=1
```
