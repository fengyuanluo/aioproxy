# ADR 0039: Provide a configuration check command

## Status
Accepted

## Context
AIOPROXY v1 uses YAML configuration with parameter-driven plugin activation. Operators need a way to validate configuration before running the long-lived service.

## Decision
AIOPROXY v1 provides a configuration check command. The check command parses and validates YAML and reports which plugins would be active, but it does not start the proxy service, does not bind listeners, and does not refresh proxy sources.

## Consequences
- Operators can validate configuration before deployment or restart.
- The command surface includes service run and config check only.
- Config check is not a status command and does not inspect runtime state.
