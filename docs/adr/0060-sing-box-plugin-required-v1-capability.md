# ADR 0060: Sing-box plugin is a required v1 capability

## Status
Accepted

## Context
AIOPROXY v1 requires FOFA, FPL, and sing-box plugins. Previous decisions established that the sing-box dependency is isolated inside the sing-box plugin, the AIOPROXY core does not depend on sing-box types, each subscription node is exposed to the core as an individual proxy candidate, and the preferred model is in-process bridging rather than allocating local ports for every node.

The sing-box plugin scope includes remote subscription URLs, local subscription files, sing-box native configurations, Clash-like YAML, base64 share-link lists, and single share links. Unsupported or malformed nodes are skipped and recorded in import reports, and the plugin may start if at least one node converts successfully.

A product boundary was needed for whether sing-box support is a formal v1 deliverable or an experimental/deferred capability.

## Decision
The sing-box plugin is a required formal v1 capability.

It is not marked experimental and is not deferred to a later version. AIOPROXY v1 is not complete unless the sing-box plugin can import supported subscription/configuration inputs, expose each successfully converted node as an independent candidate, skip unsupported nodes with import-report visibility, and participate in the same validation, scheduling, session binding, runtime failure eviction, persistence, and Admin API observability model as other candidates.

## Consequences
- Sing-box support is part of the v1 completion criteria alongside FOFA and FPL.
- The implementation must include tests or verification evidence for sing-box import and candidate scheduling behavior.
- The final delivery cannot claim v1 completion if sing-box is only a placeholder, partial stub, or documentation-only feature.
- Documentation must present sing-box as a supported v1 plugin while clearly listing unsupported node types or conversion failures through import-report behavior.
