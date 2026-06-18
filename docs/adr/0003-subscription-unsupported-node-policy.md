# ADR 0003: Skip unsupported subscription nodes with an import report

## Status
Accepted

## Context
AIOPROXY v1 must import real-world subscriptions for the sing-box plugin, including Clash-like YAML and base64 share-link lists. User-provided samples include mostly supported proxy types plus unsupported or uncertain types such as `mieru`.

## Decision
Unsupported or malformed subscription nodes are skipped rather than failing the entire plugin import. Every import must produce a structured import report that records totals, successful conversions, skipped counts, and skip reasons. The sing-box plugin may start if at least one node is successfully converted into a schedulable proxy candidate.

## Consequences
- Real-world dirty subscriptions do not prevent the service from starting.
- Unsupported nodes remain visible through import reports instead of being silently lost.
- The core pool receives only schedulable proxy candidates.
- If zero nodes are successfully converted, the plugin import fails.
