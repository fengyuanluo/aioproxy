# ADR 0005: Use configurable HTTP generate_204 validation by default

## Status
Accepted

## Context
AIOPROXY validates proxies when sources are updated. Validation must be cheap enough for large proxy imports and must work across HTTP, SOCKS5, and sing-box-backed candidates.

## Decision
The default update-time validation uses a configurable single HTTP generate_204-style URL. The default validation does not verify TLS behavior. The validation URL, accepted status codes, timeout, and concurrency must be configurable.

## Consequences
- Update-time validation stays lightweight.
- Proxies that only pass HTTP validation may still fail on HTTPS/TLS workloads at runtime.
- Runtime failure eviction handles candidates that later fail real traffic.
- Users can configure a stricter HTTPS validation URL if their workload requires it.
