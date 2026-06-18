# ADR 0025: Do not support configuration hot reload in v1

## Status
Accepted

## Context
AIOPROXY is configured through YAML. Configuration can affect listener addresses, authentication, plugin schedules, subscription sources, validation behavior, and embedded sing-box state.

## Decision
AIOPROXY v1 does not support configuration hot reload. Operators must restart the service after changing the YAML configuration.

## Consequences
- Runtime behavior is simpler and easier to reason about.
- Configuration changes have a clear restart boundary.
- SIGHUP reload and automatic file watching are out of v1 scope.
