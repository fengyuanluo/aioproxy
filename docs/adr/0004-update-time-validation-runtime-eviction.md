# ADR 0004: Validate on source update and evict on runtime failures

## Status
Accepted

## Context
AIOPROXY imports proxies from FOFA, FPL, and sing-box subscriptions. These sources can contain large numbers of nodes. Continuous background validation for every node would create persistent network and resource pressure.

## Decision
AIOPROXY validates proxies when a source is updated. Proxies that pass update-time validation enter the candidate pool. AIOPROXY does not automatically revalidate all candidates in the background after admission. During runtime, when a proxy reaches a configured failure threshold, it is marked unavailable and removed from the candidate pool.

## Consequences
- Background validation cost stays low and predictable after updates complete.
- Large imports are allowed, but update-time validation can be expensive and must have concurrency/time limits.
- A candidate can become stale between update-time validation and runtime use.
- Runtime failures are the mechanism for removing stale or broken candidates until the next source update.
