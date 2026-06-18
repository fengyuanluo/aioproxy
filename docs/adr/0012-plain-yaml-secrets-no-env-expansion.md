# ADR 0012: Use plain YAML values without environment expansion in v1

## Status
Accepted

## Context
AIOPROXY is configured entirely through YAML. Some configuration values, such as entry credentials, FOFA settings, and subscription URLs, may be sensitive.

## Decision
v1 uses plain YAML values directly and does not support environment variable expansion. Operators place required values directly in the YAML configuration file.

## Consequences
- Configuration parsing remains simple and deterministic.
- Secrets can be stored in the YAML file and must be handled carefully by the operator.
- Logs, reports, and command output should still redact sensitive values where practical.
- Environment variable interpolation may be reconsidered in a later version.
