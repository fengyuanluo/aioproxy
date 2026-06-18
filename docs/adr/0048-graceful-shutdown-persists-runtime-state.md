# ADR 0048: Graceful shutdown persists runtime state

## Status
Accepted

## Context
AIOPROXY v1 persists the candidate pool and snapshots, but does not persist session bindings. On restart it loads persisted pool state, clears sessions, and immediately refreshes active plugins.

As a long-running proxy service with a systemd deployment example, AIOPROXY needs a clear shutdown behavior for Ctrl-C, systemd stop, and upgrade restarts. Without graceful shutdown, candidate-pool and snapshot persistence would be incomplete and service restarts could unnecessarily discard useful validated proxy state.

## Decision
AIOPROXY v1 must support graceful shutdown on SIGINT and SIGTERM.

During graceful shutdown, AIOPROXY stops accepting new proxy and admin requests, gives existing proxy connections a bounded grace period to finish, persists candidate-pool and snapshot state, and exits. Session bindings are not persisted.

If existing connections do not finish within the configured grace period, AIOPROXY closes them and continues shutdown. Shutdown stages are logged.

## Consequences
- systemd stop, Ctrl-C, and upgrade restarts have predictable service behavior.
- Validated pool and snapshot state are preserved as part of the lifecycle contract.
- Session bindings remain restart-local and are cleared after restart.
- Long-lived proxy connections cannot block shutdown indefinitely because the grace period is bounded.
- The example YAML and deployment documentation must explain the graceful-shutdown grace period.
