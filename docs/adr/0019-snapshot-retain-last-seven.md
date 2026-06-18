# ADR 0019: Retain the last seven snapshots per source

## Status
Accepted

## Context
AIOPROXY v1 keeps additive updates and historical state snapshots. Retaining all snapshots indefinitely would grow unboundedly.

## Decision
AIOPROXY retains the most recent seven snapshots per source by default. The retention count is configurable, but the default is seven.

## Consequences
- Operators can inspect recent update history without unbounded storage growth.
- Older snapshots roll off automatically.
- Per-source history remains short and understandable.
