# AIOPROXY Context Glossary

This document records domain language only. It is not an implementation plan, technical specification, or task list.

## Terms

### AIOPROXY
A single-instance proxy aggregator used by a trusted operator or a small trusted client set.

### External Proxy Service
The proxy entry surface exposed by AIOPROXY to trusted clients.

### Client Credential
A static credential used to protect the External Proxy Service. It does not imply a full user, tenant, billing, or quota system.

### Session Binding
A stable mapping, for a configured lifetime, from a client session identity to one upstream proxy.

### Upstream Proxy Protocol
The normalized proxy protocol set consumed by AIOPROXY core from plugins and the pool. In v1 this is limited to HTTP CONNECT and SOCKS5.

### Sing-box Library Embedding
A mode where AIOPROXY imports sing-box Go packages and runs sing-box in-process as part of the plugin or adapter layer.

### Sing-box Sidecar
A separate sing-box process that exposes local HTTP or SOCKS5 inbound ports for AIOPROXY to consume.

### Proxy Candidate
A normalized upstream proxy candidate emitted by a plugin and consumed by the AIOPROXY core. It is not trusted as healthy until the core validates it.

### Sing-box Node Candidate
A proxy candidate derived from one node inside a sing-box subscription or configuration. Each sing-box node is visible to the AIOPROXY core as its own candidate rather than the whole sing-box plugin appearing as one proxy.

### Lazy Bridge Activation
A sing-box plugin resource model where node-specific bridge capacity is reserved or addressable, but the concrete forwarding resource is started only when the AIOPROXY core schedules that node.

### In-process Bridge
A sing-box plugin resource model where AIOPROXY forwards traffic to a sing-box node through in-process Go interfaces instead of exposing a per-node local listening port.

### Subscription Import Report
A structured report produced by a plugin after importing subscription or configuration inputs, including total nodes, successfully converted candidates, skipped nodes, and skip reasons.

### Unsupported Node
A subscription node that AIOPROXY cannot convert into a schedulable proxy candidate in the current version. Unsupported nodes are skipped and recorded in the import report.

### Candidate Pool
The runtime set of proxies that passed update-time validation and are eligible for scheduling.

### Update-time Validation
A validation pass performed when a proxy source is refreshed. Only proxies that pass this validation enter the Candidate Pool.

### Runtime Failure Eviction
A pool maintenance rule where a candidate is removed from scheduling after reaching a configured runtime failure threshold.

### Validation URL
The configurable URL used by update-time validation to decide whether a proxy can enter the Candidate Pool. The default is an HTTP generate_204 endpoint and does not validate TLS behavior.

### Runtime Failure
A failure attributed to a proxy candidate during runtime scheduling. In v1 it includes dial or handshake failure and early zero-byte upstream closure, but excludes business HTTP status codes and client aborts.

### Explicit Session Identifier
A client-provided identifier that activates Session Binding. It is distinct from the Client Credential username and does not participate in authentication.

### Non-session Scheduling
The configured scheduling behavior used when a request does not include an explicit session identifier, such as random or round-robin.

### Session Username Expression
A proxy authentication username that encodes an explicit session using `<credential>-<session>` or `<credential>-<session>-<ttl>`. The credential part cannot contain `-`; the session part may contain `-`; a trailing duration token is treated as session lifetime.

### FOFA Source Query
A configured FOFA search request used by the FOFA plugin to produce proxy candidates. It declares the FOFA query, output protocol, requested fields, and pagination settings.

### Read-only Admin API
A local-only administrative HTTP surface that exposes health, statistics, pool state, snapshots, and import reports without providing mutation endpoints.

### Snapshot
An immutable retained record of a source refresh or candidate-pool state.

### Additive Update
A refresh model where newly validated proxy candidates are merged into the current candidate pool instead of replacing the pool.

### Canonical Proxy Fingerprint
The global deduplication identity for a proxy candidate. For HTTP and SOCKS5 candidates it includes normalized protocol, host, port, and credential material; for sing-box node candidates it also includes a stable node identifier or outbound tag plus a config hash.

### Snapshot Retention
The per-source policy that keeps only the most recent retained snapshots. In v1 the default retention is 7 snapshots per source.

### Scheduled Refresh
A plugin-driven refresh that happens on a timer rather than through a mutation endpoint. Different plugins may use different refresh intervals.

### Persistent Candidate Pool
A saved candidate-pool state that is loaded on service restart so previously validated candidates remain available before or alongside new scheduled refreshes.

### Concurrency Acceptance Gate
A v1 completion criterion requiring AIOPROXY to pass a 300-concurrent-client stress scenario without process crash, panic, or deadlock before the release can be considered complete.

### Degraded Health
An observable service state where AIOPROXY is running but at least one configured proxy source or plugin is failing, empty, or otherwise unable to contribute candidates as expected.

### Plugin Degradation
A plugin state indicating that the plugin is configured but its latest refresh failed or produced no usable candidates.

### Basic Admin API View
A read-only administrative response shape that exposes only basic operational information. It is not a raw debug dump and does not include secret-bearing source material or full proxy node definitions.

### Graceful Shutdown
A service lifecycle behavior where AIOPROXY stops accepting new work after a shutdown signal, gives in-flight proxy connections a bounded time to finish, persists candidate-pool and snapshot state, and exits without persisting session bindings.

### No Active Plugin State
A service state where no configured proxy-source plugin is active, leaving AIOPROXY without a live source for new proxy candidates.
