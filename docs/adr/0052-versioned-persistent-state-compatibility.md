# ADR 0052: Version persisted state and protect v1 upgrades

## Status
Accepted

## Context
AIOPROXY v1 persists the candidate pool and snapshots, reloads them on restart, and saves them during graceful shutdown. Session bindings are not persisted. Because AIOPROXY is released as long-lived binaries and may be upgraded in place, the product needs a clear boundary for how persisted internal state behaves across v1 upgrades.

The persisted files are internal runtime state rather than a public API, but silently corrupting, discarding, or crashing on old state would make upgrades operationally fragile.

## Decision
AIOPROXY v1 persisted candidate-pool and snapshot files must include a schema or format version.

Within the v1 series, AIOPROXY should read older persisted state compatibly when practical. If persisted state is incompatible or cannot be parsed safely, AIOPROXY backs up the old state files, rebuilds a fresh empty state, continues startup, reports degraded health as appropriate, and waits for plugin refreshes to repopulate candidates.

Persisted state files are not a public stable API. The compatibility promise is an operational upgrade safeguard for the v1 product line, not an external integration contract.

## Consequences
- Upgrades within v1 are less likely to break service startup because of old state files.
- Operators retain a backup of incompatible state for troubleshooting.
- Startup does not crash solely because persisted state cannot be loaded.
- A rebuild from incompatible state may temporarily leave the Candidate Pool empty, causing degraded health and empty-pool fast failure until refreshes succeed.
- Documentation must explain that persisted files are versioned internal state, not a supported external API.
