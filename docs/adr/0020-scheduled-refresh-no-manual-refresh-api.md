# ADR 0020: Use scheduled refresh only, with no manual refresh API

## Status
Accepted

## Context
AIOPROXY v1 exposes a read-only admin surface and keeps additive candidate-pool updates. The user clarified that refresh is not triggered through an API endpoint; it happens only on a schedule, and different plugins may refresh at different intervals.

## Decision
AIOPROXY v1 does not expose a manual refresh endpoint. Refreshes are scheduled only. Each plugin may have its own refresh interval.

## Consequences
- The admin API remains read-only.
- Refresh behavior is predictable and automatic.
- Plugin configuration must include its schedule interval.
- Operations that need immediate refresh must be handled outside the admin API.
