# ADR 0024: Fail fast when the candidate pool is empty

## Status
Accepted

## Context
AIOPROXY can start in a degraded state and the candidate pool can become empty if sources fail or all candidates are evicted. External proxy clients still need deterministic behavior.

## Decision
When the candidate pool has no schedulable candidates, external proxy requests fail immediately. HTTP proxy requests return a service-unavailable style failure. SOCKS5 requests return a general failure. Requests do not wait for future refreshes.

## Consequences
- Clients fail quickly instead of hanging.
- The service avoids connection buildup while empty.
- Operators can inspect read-only admin status and logs to determine why the pool is empty.
