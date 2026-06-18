# ADR 0045: Plugin degradation affects health but not service startup

## Status
Accepted

## Context
AIOPROXY v1 refreshes every active plugin once at startup and then continues scheduled refreshes. Previous decisions established that startup refresh failure does not stop the service, and that empty candidate pools fail proxy requests fast.

The remaining product boundary is how a configured plugin with a failed refresh or zero usable imported candidates affects the overall service health.

## Decision
AIOPROXY continues running when a configured plugin fails refresh or imports zero usable candidates.

That plugin is marked degraded. The overall Admin API health is reported as degraded rather than healthy while any configured plugin is degraded.

If the Candidate Pool still contains usable candidates from persisted state or other plugins, the External Proxy Service continues serving requests. If the Candidate Pool is empty, proxy requests fail fast according to the empty-pool behavior.

## Consequences
- A temporary source or plugin failure does not take the whole proxy service offline.
- Monitoring can distinguish fully healthy operation from degraded operation.
- Operators can inspect plugin status and import reports through the read-only Admin API.
- The service must not claim healthy status while configured sources are failing or contributing zero usable candidates.
