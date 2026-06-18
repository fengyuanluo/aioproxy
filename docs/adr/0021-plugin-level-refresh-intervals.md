# ADR 0021: Configure refresh intervals at plugin level

## Status
Accepted

## Context
AIOPROXY refreshes proxy sources on a schedule and does not expose manual refresh endpoints in v1. Different plugins may need different refresh frequencies.

## Decision
Refresh intervals are configured at the plugin level. FOFA, FPL, and sing-box each have one refresh interval. Sources or queries inside a plugin refresh together on that plugin's schedule.

## Consequences
- Scheduling remains easy to understand and configure.
- Plugin refresh behavior matches the product language: different plugins can refresh at different times.
- Per-source or per-query refresh schedules are out of v1 scope.
