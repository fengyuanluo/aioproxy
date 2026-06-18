# ADR 0035: Do not build a Web UI in v1

## Status
Accepted

## Context
AIOPROXY v1 already provides a read-only admin API and a Chinese-first documentation library. The core product goal is a high-performance proxy aggregator with plugin-based sources.

## Decision
AIOPROXY v1 does not include a Web UI. Operational visibility is provided by the read-only admin API and logs.

## Consequences
- v1 remains focused on proxy service and source integration.
- No frontend build pipeline is required.
- A Web UI can be considered later if the admin API proves useful enough to support one.
