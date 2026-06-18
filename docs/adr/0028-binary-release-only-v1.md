# ADR 0028: Release multi-platform binaries only in v1

## Status
Accepted

## Context
AIOPROXY v1 requires GitHub Actions builds for multi-platform binaries. Docker image distribution was not required for v1.

## Decision
AIOPROXY v1 publishes multi-platform binary artifacts only. Docker image publishing is out of v1 scope.

## Consequences
- The release pipeline remains focused on Go binary builds.
- Operators run the binary directly with a YAML configuration file and persistent data directory.
- Docker packaging can be added later if needed.
