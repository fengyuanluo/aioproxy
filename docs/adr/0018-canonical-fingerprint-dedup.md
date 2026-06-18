# ADR 0018: Use a canonical proxy fingerprint for global deduplication

## Status
Accepted

## Context
AIOPROXY uses additive updates and retains snapshots. The candidate pool must avoid duplicate entries while still allowing repeated source refreshes and multi-source ingestion.

## Decision
AIOPROXY deduplicates candidates using a global canonical proxy fingerprint. For HTTP and SOCKS5 candidates, the fingerprint includes normalized protocol, host, port, and credential material. For sing-box node candidates, the fingerprint additionally includes a stable node identifier or outbound tag plus a config hash. Source identity, snapshot identity, and timestamps do not participate in the fingerprint.

## Consequences
- The same proxy from different sources collapses to one candidate.
- Different credentials on the same host and port remain distinct candidates.
- sing-box nodes remain individually identifiable.
- Source and snapshot metadata stay attached separately for auditability.
