# ADR 0050: Allow startup with no active plugins

## Status
Accepted

## Context
AIOPROXY v1 uses parameter-driven plugin activation. A plugin may be inactive because its configuration block is absent or because the required activation parameters are empty. Previous decisions established that startup refresh failures do not stop the service, plugin degradation affects health but not startup, and an empty Candidate Pool fails proxy requests fast.

A product boundary was needed for the case where no proxy-source plugin is active at all.

## Decision
AIOPROXY v1 allows the service to start when there are no active plugins.

When no active plugins exist, the service reports degraded health. If the Candidate Pool is empty, proxy requests fail fast according to the existing empty-pool behavior. The Admin API should make the no-active-plugin state observable through health, plugin status, and pool statistics.

## Consequences
- Operators may start the service before configuring proxy sources.
- Configuration omissions do not prevent the process from starting.
- The service does not claim healthy status when it has no active proxy-source plugins.
- The empty-pool fast-failure behavior remains the external proxy behavior when no candidates are available.
- Documentation and `check` output must clearly indicate when no plugins would be active.
