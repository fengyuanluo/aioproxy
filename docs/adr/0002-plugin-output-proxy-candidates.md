# ADR 0002: Plugins output proxy candidates and core owns pool behavior

## Status
Accepted

## Context
AIOPROXY integrates multiple proxy sources: FOFA, FPL, and embedded sing-box. The core must remain responsible for health checks, scheduling, session binding, deduplication, and pool maintenance.

## Decision
Plugins output proxy candidates. A proxy candidate is an upstream proxy candidate with normalized protocol and endpoint metadata. The AIOPROXY core owns deduplication, health checking, scheduling, session binding, failure handling, and pool maintenance.

For sing-box subscriptions or configurations, each sing-box node must be represented to the AIOPROXY core as a separate proxy candidate. The sing-box plugin must not appear to the core as one opaque aggregate proxy.

## Consequences
- Pool behavior remains consistent across FOFA, FPL, and sing-box sources.
- The core can score and schedule individual sing-box nodes.
- The sing-box plugin must expose or bridge individual nodes into normalized HTTP CONNECT or SOCKS5 candidates.
- Plugin-specific health or node selection must not replace core pool scheduling in v1.
