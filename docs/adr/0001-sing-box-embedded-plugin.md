# ADR 0001: Embed sing-box only behind the sing-box plugin boundary

## Status
Accepted

## Context
AIOPROXY v1 needs a sing-box integration that can consume one or more sing-box subscriptions or configuration files and expose proxies to the AIOPROXY core. The core proxy pool is already constrained to consume normalized HTTP CONNECT and SOCKS5 upstream proxies.

## Decision
AIOPROXY will use sing-box as an embedded Go library for the sing-box plugin path. The sing-box dependency must be isolated behind the sing-box plugin boundary. The AIOPROXY core must interact with it only through the common plugin/provider protocol, not through sing-box-specific types.

## Consequences
- The core remains decoupled from sing-box internals.
- The sing-box plugin may carry heavier dependencies than other plugins.
- Build, lifecycle, cancellation, and health behavior for embedded sing-box must be validated as part of the sing-box plugin acceptance path.
- Other plugins must not depend on sing-box packages.
