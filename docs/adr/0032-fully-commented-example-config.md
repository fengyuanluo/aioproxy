# ADR 0032: Provide a fully commented example YAML configuration

## Status
Accepted

## Context
AIOPROXY v1 is configured entirely through YAML. Users need a clear product-level reference for available settings and plugin activation behavior.

## Decision
AIOPROXY v1 includes a complete example YAML configuration file. Every parameter in the example must include comments that explain its purpose and behavior.

## Consequences
- Users can start from a documented configuration template.
- Product behavior is discoverable without reading source code.
- The example configuration must stay in sync with supported v1 options.
