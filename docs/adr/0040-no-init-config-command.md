# ADR 0040: Do not provide an init-config command in v1

## Status
Accepted

## Context
AIOPROXY v1 includes a fully commented example YAML configuration file. The command surface should stay small.

## Decision
AIOPROXY v1 does not provide an `init-config` or default-configuration generation command. Users start from the documented example YAML file.

## Consequences
- The command surface remains limited to serving and configuration checking.
- The example YAML is the canonical configuration template.
- Future versions can add config generation if needed.
