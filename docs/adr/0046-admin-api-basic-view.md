# ADR 0046: Admin API returns only basic operational information

## Status
Accepted

## Context
AIOPROXY v1 provides a read-only Admin API for health, statistics, pool state, snapshots, import reports, and plugin status. The Admin API may be unauthenticated on loopback, and previous decisions require token configuration when it is exposed beyond loopback.

A product boundary is needed for what read-only responses may contain. Returning full proxy definitions, subscription material, FOFA keys, proxy passwords, or sing-box share links would make the Admin API a debugging dump and increase accidental disclosure risk.

## Decision
The Admin API returns only basic operational information.

Basic fields are returned as-is rather than masked. Examples include protocol, host, port, source name, status, counters, timestamps, validation result summaries, and skip/error categories.

Secret-bearing or raw source material is not part of the Admin API response model. This includes FOFA keys, proxy passwords, full subscription URLs with embedded tokens, raw sing-box node definitions, raw share links, and full source configuration payloads.

Detailed debugging is performed through the file logs rather than by expanding the Admin API into a full debug endpoint.

## Consequences
- Admin API responses stay small and operationally focused.
- Basic routing and pool information can be inspected without artificial masking.
- Sensitive source material is excluded by design rather than returned in redacted form.
- Operators who need deeper debugging inspect log files instead of expecting full raw objects from the Admin API.
- Documentation must clearly distinguish Admin API basic operational views from file-log debugging.
