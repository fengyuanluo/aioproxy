# ADR 0026: Persist candidate pool and snapshots, not sessions

## Status
Accepted

## Context
AIOPROXY uses additive updates, snapshot retention, runtime failure eviction, and explicit session binding. Restart behavior must be predictable.

## Decision
AIOPROXY v1 persists the candidate pool and snapshots across restarts. Session bindings are not persisted. On restart, AIOPROXY loads the persisted candidate pool and retained snapshots, clears sessions, and starts plugin refresh schedules including the immediate startup refresh.

## Consequences
- Previously validated candidates can be available after restart.
- Historical snapshots survive restarts.
- Sessions remain runtime-only and are recreated by subsequent explicit session requests.
- Stale persisted candidates are removed later by runtime failure eviction or future refresh/validation behavior.
