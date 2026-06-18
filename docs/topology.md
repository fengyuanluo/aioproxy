# AIOPROXY 拓扑

## 总体架构

```mermaid
flowchart LR
  Client[HTTP/SOCKS5 Client] --> Mixed[Mixed Proxy Listener]
  Mixed --> Auth[Auth + Session Parser]
  Auth --> Scheduler[Scheduler + Session Binding]
  Scheduler --> Pool[Candidate Pool]
  Pool --> HTTP[HTTP Upstream]
  Pool --> SOCKS[SOCKS5 Upstream]
  Pool --> SB[sing-box Node Dialer]
  FPL[FPL Plugin] --> Validate[Update-time Validation]
  FOFA[FOFA Plugin] --> Validate
  SING[sing-box Plugin] --> Validate
  Validate --> Pool
  Pool --> Store[(Persistent Pool/Snapshots)]
  Admin[Read-only Admin API] --> Pool
  Admin --> Store
```

## 刷新与入池

```mermaid
sequenceDiagram
  participant Plugin
  participant Validator
  participant Pool
  participant Store
  Plugin->>Plugin: fetch/parse source
  Plugin->>Validator: Proxy candidates
  Validator->>Validator: HTTP generate_204 validation
  Validator->>Pool: add validated candidates
  Validator->>Store: save snapshot + pool
```

## 请求调度

```mermaid
flowchart TD
  Req[Client Request] --> Parse[Protocol + Auth]
  Parse --> Sess{Session ID?}
  Sess -- yes --> Bind[Find/Rebind Candidate]
  Sess -- no --> Policy[random/round_robin]
  Bind --> Dial[Dial Upstream]
  Policy --> Dial
  Dial --> Tunnel[Bidirectional Tunnel]
  Tunnel --> Fail{Runtime failure?}
  Fail -- yes --> Evict[Failure count / eviction]
  Fail -- no --> OK[Keep candidate available]
```

## sing-box 节点桥接

```mermaid
flowchart LR
  Source[Subscription / Config Source] --> Parser[sing-box Plugin Parser]
  Parser --> Convert[Node to sing-box Outbound]
  Convert --> OneBox[Per-node in-process Box]
  OneBox --> Dialer[Outbound Dialer]
  Dialer --> Candidate[Sing-box Node Candidate]
  Candidate --> Core[Core Pool]
  Core --> Schedule[Scheduling]
  Schedule --> Dialer
  Convert -->|unsupported or build failed| Report[Import Report Skip Reason]
```

说明：sing-box 依赖只在插件内使用；主体只看到标准 candidate 与 dialer，不依赖 sing-box 类型。每个可启动节点对应独立候选，不为每个节点预注册本地端口。

## 持久化与快照

```mermaid
flowchart TD
  Refresh[Plugin Refresh] --> Validate[Update-time Validation]
  Validate --> Pool[Candidate Pool]
  Pool --> PoolFile[(data/pool.json)]
  Refresh --> Report[Import Report]
  Report --> Snapshot[(data/snapshots/source/*.json)]
  Shutdown[SIGINT/SIGTERM] --> Grace[Graceful Shutdown]
  Grace --> PoolFile
  Restart[Restart] --> Load[Load Versioned Pool]
  Load -->|compatible| Pool
  Load -->|incompatible| Backup[Backup Old State]
  Backup --> Empty[Rebuild Empty State]
```

说明：sessions 不持久化；持久化状态带版本，不兼容时备份旧文件并继续启动。

## Admin API 可观测面

```mermaid
flowchart LR
  Operator[Operator] --> Admin[Read-only Admin API]
  Admin --> Health[/health]
  Admin --> Stats[/stats]
  Admin --> PoolView[/pool]
  Admin --> Plugins[/plugins]
  Admin --> Snapshots[/snapshots]
  Health --> Pool[Pool Counts]
  Health --> PluginState[Plugin Degradation]
  Plugins --> Reports[Import Reports]
  PoolView --> Basic[Basic Candidate Info]
```

说明：Admin API 不提供刷新/删除/修改；只返回基础运行信息，不返回 FOFA key、代理密码、完整订阅 URL 或 raw node。

## 发布构建流

```mermaid
flowchart TD
  Push[push main] --> Test[go test]
  PR[pull_request] --> Test
  Test --> Matrix[Cross Build Matrix]
  Matrix --> Artifacts[CI Artifacts]
  Tag[v* tag] --> Test
  Tag --> ReleaseBuild[Build + Package]
  ReleaseBuild --> Checksums[Checksums]
  Checksums --> GHRelease[GitHub Release Assets]
```

说明：main push 产出 CI artifacts；`v*` tag 才创建正式 GitHub Release。v1 发布 Linux/macOS/Windows 的 amd64 与 arm64 二进制包。
