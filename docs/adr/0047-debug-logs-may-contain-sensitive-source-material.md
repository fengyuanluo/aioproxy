# ADR 0047: Debug logs may contain sensitive source material

## Status
Accepted

## Context
AIOPROXY v1 keeps the Admin API as a basic operational view rather than a raw debug dump. Deep troubleshooting is performed through file logs. The remaining product boundary is whether debug-level file logs may include full sensitive source material.

Sensitive material can include FOFA keys, proxy passwords, full subscription URLs, sing-box node definitions, share links, and source configuration fragments. Allowing this information in logs improves direct troubleshooting but increases the risk of accidental disclosure through retained, rotated, compressed, copied, or shared log files.

## Decision
AIOPROXY debug-level logs may include complete sensitive source material when it is useful for troubleshooting.

Normal operational logging should remain less verbose. The product documentation must warn that enabling debug logging can write secrets and complete proxy/subscription/node material to disk.

## Consequences
- Deep troubleshooting can be performed from log files without expanding the Admin API into a raw debug endpoint.
- Operators must treat debug logs as sensitive artifacts.
- Support reports, issue attachments, and final summaries must avoid pasting raw debug logs unless secrets are manually removed.
- Log rotation and compression can retain sensitive material, so retention settings become part of the operational security posture.
- Documentation must clearly distinguish normal logs from debug logs that may contain secrets.
