# ADR 0027: Provide only the service command in v1

## Status
Accepted

## Context
AIOPROXY v1 already exposes a read-only admin API for health, statistics, pool state, snapshots, and import reports.

## Decision
AIOPROXY v1 does not provide additional read-only CLI status commands. The v1 binary focuses on running the service. Operational status is inspected through the read-only admin API.

## Consequences
- The command surface remains small.
- Admin API remains the single status inspection surface.
- CLI status, pool, and snapshot inspection commands can be added later if needed.
