# ADR 0006: Count dial, handshake, and early zero-byte failures at runtime

## Status
Accepted

## Context
AIOPROXY evicts runtime candidates after a configured number of failures. Counting every failed client request would incorrectly evict proxies for target-site errors or client behavior.

## Decision
Runtime failure counting includes proxy-attributable failures: dial errors, proxy handshake errors, sing-box plugin dialer errors, and early upstream closure with zero transferred bytes inside a configured early-failure window. Business HTTP response status codes and client aborts do not count as proxy failures.

## Consequences
- Candidate eviction is tied to proxy usability rather than target-site behavior.
- The forwarding layer must track whether useful bytes were transferred during the early-failure window.
- Some degraded proxies that fail only after transferring bytes may remain until later failures occur.
