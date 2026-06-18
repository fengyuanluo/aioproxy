# ADR 0038: Use relative data and log directories by default

## Status
Accepted

## Context
AIOPROXY v1 runs as a binary service with file logging and persisted candidate pool/snapshots. Default paths should be easy for local operation.

## Decision
AIOPROXY uses relative directories under the current working directory by default, such as `./data` for persisted state and `./logs` for log files.

## Consequences
- Local binary execution is simple and self-contained.
- Operators running under systemd must set a clear working directory or override paths in YAML.
- The systemd example and documentation must call out the working-directory dependency.
