# ADR 0036: Support configurable log format, level, and compressed rotation

## Status
Accepted

## Context
AIOPROXY v1 runs as a long-lived proxy service. Operators need readable development logs and production-friendly structured logs, while preventing unbounded log growth.

## Decision
AIOPROXY v1 logs in text format by default and supports JSON log format through YAML. Log level is configurable. Log output supports automatic rotation and compression.

## Consequences
- Default logs remain human-readable.
- JSON logs can be used for structured collection.
- Long-running deployments avoid unbounded log file growth.
- The example YAML must document log format, level, rotation, and compression settings.
