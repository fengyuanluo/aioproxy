# ADR 0023: Startup refresh failures degrade but do not stop the service

## Status
Accepted

## Context
AIOPROXY refreshes enabled plugins immediately on startup. Proxy sources can be unreliable, credentials can be wrong, and validation can result in no usable candidates.

## Decision
Plugin refresh failures during startup do not prevent the external proxy service and read-only admin API from starting. Failed plugins are reported as degraded through logs and admin state. Scheduled refresh continues to retry later.

## Consequences
- AIOPROXY can start even when some or all sources are temporarily unavailable.
- The candidate pool may be empty after startup.
- Operators must use read-only status/logs to see degraded plugin state.
- Later scheduled refreshes can recover without restarting the service.
