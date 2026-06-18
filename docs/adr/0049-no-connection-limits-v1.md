# ADR 0049: Do not implement connection limits in v1

## Status
Accepted

## Context
AIOPROXY v1 is a single-instance proxy aggregator for a trusted operator or a small trusted client set. It must pass the 300-concurrent-client acceptance gate, but it is not intended to be a multi-tenant proxy gateway with quotas or fairness controls.

A product boundary was needed for whether v1 should include global maximum connections, per-client maximum connections, or idle-time resource-protection policy as first-class user-facing behavior.

## Decision
AIOPROXY v1 does not implement connection-level limiting or protection as a product feature.

There is no v1 requirement for global maximum concurrent connections, per-client maximum concurrent connections, or quota-like connection fairness controls. The 300-concurrent-client stress gate remains required and is not replaced by connection limiting.

## Consequences
- v1 stays simpler and avoids introducing quota/fairness semantics.
- A single trusted client can consume available service capacity.
- Stability must come from robust proxy handling and successful stress validation rather than rejecting traffic through configured connection caps.
- Future versions may add connection protection if the product expands beyond trusted self-use or small trusted client sets.
