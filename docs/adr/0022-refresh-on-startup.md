# ADR 0022: Refresh enabled plugins immediately on startup

## Status
Accepted

## Context
AIOPROXY refreshes plugins on plugin-level schedules and does not expose manual refresh endpoints in v1. Waiting for the first timer interval would delay initial proxy availability.

## Decision
Each enabled plugin performs one refresh immediately during service startup, then continues on its configured plugin-level refresh interval.

## Consequences
- The service can populate its candidate pool shortly after startup.
- Startup can take longer because refresh and validation happen immediately.
- Operators can still tune refresh intervals independently per plugin.
