# ADR 0037: Log to files by default instead of stdout/systemd

## Status
Accepted

## Context
AIOPROXY v1 runs as a long-lived service and supports automatic log rotation and compression. The user prefers not to rely on systemd/journald output for normal logging.

## Decision
AIOPROXY logs to files by default. It does not use stdout/systemd logging as the default operational log sink. File logs support automatic rotation and compression.

## Consequences
- Operators inspect AIOPROXY log files directly.
- The systemd example should avoid depending on journald for normal application logs.
- Log file paths and rotation settings must be documented in the example YAML.
