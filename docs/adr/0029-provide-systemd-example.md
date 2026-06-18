# ADR 0029: Provide a systemd service example

## Status
Accepted

## Context
AIOPROXY v1 is distributed as multi-platform binaries rather than Docker images. Server operators still need a practical deployment reference.

## Decision
AIOPROXY v1 includes a systemd service example. It is documentation/packaging guidance, not a mandatory installer.

## Consequences
- Linux server deployment is easier.
- The release remains binary-focused.
- Operators can adapt the unit file to their paths and environment.
