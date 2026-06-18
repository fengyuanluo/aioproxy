# ADR 0042: Require token only when Admin API is exposed beyond local loopback

## Status
Accepted

## Context
AIOPROXY v1 provides a local-only read-only Admin API for health, statistics, pool state, snapshots, import reports, and plugin status. The Admin API has no mutation endpoints in v1. Operators need convenient local observability without weakening the security boundary if the Admin API is exposed to a non-local address.

## Decision
The Admin API is unauthenticated only when it listens on a loopback address such as `127.0.0.1` or `::1`.

If the Admin API is configured to listen on a non-loopback address, AIOPROXY requires an explicit Admin API token configuration and refuses to start without it.

The Admin API credential is separate from the External Proxy Service username/password.

## Consequences
- Local read-only status checks remain simple.
- Accidental remote exposure without an Admin API token fails at startup.
- The proxy-service credential and the management-surface credential remain separate concepts.
- The example YAML and documentation must explain the loopback/no-token and non-loopback/token startup rule.
