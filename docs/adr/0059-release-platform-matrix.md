# ADR 0059: Release Linux, macOS, and Windows binaries for amd64 and arm64

## Status
Accepted

## Context
AIOPROXY v1 is distributed as multi-platform binaries through GitHub Actions. Docker images are out of scope for v1. The release workflow builds CI artifacts on main pushes and formal release assets on version tags.

A product boundary was needed for which operating systems and CPU architectures are included in the v1 binary release matrix.

## Decision
AIOPROXY v1 builds and releases binaries for the following target platforms:

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`
- `windows/arm64`

Additional targets such as `linux/386`, ARMv7, or FreeBSD are out of scope for v1.

## Consequences
- The v1 release covers mainstream server, macOS, and Windows environments.
- The release matrix remains moderate in size.
- GitHub Actions must package platform-specific archives and checksums for these six targets.
- Documentation must list the supported binary targets and avoid implying support for unbuilt platforms.
