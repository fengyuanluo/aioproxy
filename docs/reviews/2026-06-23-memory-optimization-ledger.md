# AIOPROXY idle memory optimization ledger

Date: 2026-06-23
Scope: reduce idle memory usage for FPL + sing-box + region-validation deployments by shifting heavy sing-box resources from refresh-time permanent allocation to request-time/on-demand allocation, and by forcing OS memory return after bulk validation cleanup.

## Baseline evidence

Repro fixture used a local-only, deterministic workload to avoid external-source drift:

- 200 sing-box `direct` outbounds in `tmp/memtest-src/singbox.yaml`.
- Local config `tmp/memtest-config.yaml` with FPL block plus sing-box file source and `validation.skip_validation=true`.
- Built current pre-change binary as `tmp/aioproxy-mem-before`.

Baseline command excerpt:

```bash
GOFLAGS=-mod=readonly go build -o tmp/aioproxy-mem-before ./cmd/aioproxy
./tmp/aioproxy-mem-before serve -c tmp/memtest-config.yaml
ps -p "$pid" -o pid,ppid,rss,vsz,pmem,comm,args
cat /proc/$pid/status | egrep 'Name|VmRSS|VmHWM|VmData|RssAnon|RssFile|Threads'
```

Baseline result after startup refresh, before any user proxy request:

```text
RSS=113016 kB
VmHWM=113016 kB
VmRSS=113016 kB
RssAnon=99240 kB
VmData=408292 kB
Threads=29
plugin refresh finished plugin=singbox imported=200 validated=200 degraded=false
```

Interpretation: merely importing sing-box candidates permanently started one `box.Box` per node and kept all boxes resident while the service was idle.

## Findings and repair ledger

| ID | Status | Area | Finding | Repair | Verification |
|---|---|---|---|---|---|
| MEM-001 | fixed | `internal/plugins/singbox` | `Plugin.Refresh()` eagerly built and started a sing-box `box.Box` per converted node. Every node stayed resident in `p.boxes` before any user request. | Replaced eager `outboundDialer`/`p.boxes` with a lightweight `lazyOutboundDialer` that stores only the per-node JSON config at refresh time. The box starts only when `DialContext` is called. | `go test ./internal/plugins/singbox -count=1`; memtest skip-validation RSS dropped from 113016 kB to 30204 kB after importing 200 nodes. |
| MEM-002 | fixed | `internal/plugins/singbox` | Old `p.boxes` map only replaced boxes with the same fingerprint and did not remove boxes for disappeared nodes, allowing stale sing-box resources to accumulate across refreshes. | Removed plugin-level `p.boxes`; persistent per-node resources now live only inside lazy dialers and are closed by idle timer/reset. | Diff removes `mu/boxes/closeBoxes`; `go test ./...`; `go vet ./...`. |
| MEM-003 | fixed | `internal/app` + validation | Region/IP validation can temporarily instantiate many sing-box boxes through dialers. Without explicit cleanup and OS-memory return, idle RSS can remain close to validation peak after refresh. | After `Validator.Validate`, `pluginManager.refresh` calls `ResetIdleCache()` on dialers that support it and then `debug.FreeOSMemory()` once per refresh batch. | Non-skip validation test with 200 nodes: `VmHWM=113180 kB`, post-refresh `VmRSS=38920 kB`, proving peak allocations are released after validation. |
| MEM-004 | fixed | `internal/plugins/singbox` request path | Request-time sing-box boxes need to stay alive while returned connections are active, but should not remain forever after traffic stops. | `lazyOutboundDialer` wraps returned `net.Conn`, tracks active connections, schedules close after 30s idle, and coalesces `debug.FreeOSMemory()` after idle close. | Live smoke: after refresh `RSS=26868 kB`; immediately after one request `RSS=28628 kB`; after idle TTL + free `RSS=27568 kB`. |
| MEM-005 | fixed | `internal/plugins/singbox` config normalization | `direct`/`block` outbound configs could carry `server/server_port` from candidate metadata into sing-box JSON, causing `json: unknown field "server"` when lazy direct outbounds are started. | `cleanOutboundMap` now strips connection/protocol-specific fields for `direct` and `block`. | `TestLazySingBoxDialerCanResetAndRecreate` reproduces lazy direct outbound start, reset, and second dial. |
| MEM-006 | reviewed / no code change | FPL parser and candidate pool | FPL uses streaming scanner and stores only normalized `Candidate` values. It does not keep per-candidate goroutines, transports, or dialers. Candidate metadata is small compared with sing-box boxes. | No change in this pass. Main idle RSS source was sing-box permanent boxes. | Code review: `internal/plugins/fpl/fpl.go` parses stream into candidates only; memtest after change remains low even with FPL block present. |
| MEM-007 | reviewed / no code change | plugin scheduler | Idle goroutine cost is one refresh loop per active plugin plus listener goroutines. This is not the 400MB-class RSS source. | No change in this pass to preserve ADR 0021/0022 scheduled-refresh semantics. | Runtime after optimized refresh with 200 nodes: Threads=13 in skip-validation idle test; Threads=41 only during/after validation workload. |

## After-change memory evidence

### A. Idle import with 200 sing-box nodes, no validation

Command:

```bash
GOFLAGS=-mod=readonly go build -o tmp/aioproxy-mem-after ./cmd/aioproxy
./tmp/aioproxy-mem-after serve -c tmp/memtest-config.yaml
```

Result after refresh, before any user request:

```text
RSS=30204 kB
VmHWM=30204 kB
VmRSS=30204 kB
RssAnon=15504 kB
VmData=184548 kB
Threads=13
plugin refresh finished plugin=singbox imported=200 validated=200 degraded=false
```

Reduction against baseline: `113016 kB -> 30204 kB`, about `73.3%` lower RSS for the same 200-node idle import workload.

### B. Region-style validation with 200 sing-box nodes

Command:

```bash
python3 -m http.server 19999 -d tmp
./tmp/aioproxy-mem-after serve -c tmp/memtest-validate-config.yaml
```

Result after validation and cleanup:

```text
VmHWM=113180 kB
VmRSS=38920 kB
RssAnon=22496 kB
VmData=516812 kB
Threads=41
plugin refresh finished plugin=singbox imported=200 validated=186 degraded=false
```

Interpretation: validation still has a temporary peak because each candidate needs a real dial attempt, but the process no longer stays at peak when idle.

### C. Real proxy smoke through lazy sing-box direct outbound

Command:

```bash
./tmp/aioproxy-lazy-smoke serve -c tmp/lazy-smoke-config.yaml
curl -fsS -x http://aio:change-me@127.0.0.1:19380 http://127.0.0.1:19391/ >/dev/null
```

Result:

```text
after refresh: RSS=26868 kB
after request: RSS=28628 kB
after idle ttl + free: RSS=27568 kB
```

Interpretation: the lazy dialer starts a box on first request, serves the request, then closes the idle box after 30s and requests OS memory release.

## Validation commands run

```bash
GOFLAGS=-mod=readonly go test ./internal/plugins/singbox -count=1 -v
GOFLAGS=-mod=readonly go test ./...
go vet ./...
go run ./cmd/aioproxy check -c examples/config.yaml
go run ./cmd/aioproxy check -c tmp/memtest-validate-config.yaml
```

Key passing output:

```text
ok   github.com/aioproxy/aioproxy/internal/plugins/singbox 0.039s
ok   github.com/aioproxy/aioproxy/internal/plugins/singbox 0.036s
AIOPROXY config check
active plugins: singbox
result: ok
```

## Residual risk / next optimization candidates

1. Validation peak memory is still proportional to validation concurrency and node count. It is no longer idle-resident, but deployments with thousands of sing-box nodes can still see temporary refresh peaks. If needed, add a sing-box-specific validation concurrency cap lower than global validation concurrency.
2. Go RSS can remain a few MB above pre-request baseline after a request due runtime heap/span behavior even after `debug.FreeOSMemory()`. The large permanent idle allocation has been removed.
3. FPL candidate count still consumes memory linearly as `Candidate` structs in the pool, by design. This is small compared with sing-box boxes and preserves Admin API / snapshot semantics.
