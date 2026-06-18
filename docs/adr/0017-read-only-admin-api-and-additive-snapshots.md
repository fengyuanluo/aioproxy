# ADR 0017: Use a read-only local admin API and additive update snapshots

## Status
Accepted

## Context
AIOPROXY v1 needs observability for health, statistics, candidate pools, import reports, and historical update state. The user also requires that updates keep snapshots and add validated proxies instead of overwriting the current proxy set.

## Decision
AIOPROXY v1 exposes a local-only read-only admin API. The admin surface provides inspection of health, statistics, candidate pools, snapshots, and import reports. It does not provide mutation endpoints in v1.

Source refreshes are additive: newly validated proxy candidates are merged into the current candidate pool instead of replacing it. Historical snapshots of update results are retained for inspection.

## Consequences
- Operators can inspect current and historical state without mutating the service.
- Candidate pools are not destroyed on refresh.
- Snapshot retention policy must be defined separately to prevent unbounded growth.
- Future versions may add write endpoints if needed, but v1 remains read-only.
