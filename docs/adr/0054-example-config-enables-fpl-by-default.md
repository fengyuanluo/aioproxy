# ADR 0054: Example config enables FPL by default

## Status
Accepted

## Context
AIOPROXY v1 requires a fully commented YAML example configuration and does not provide an `init-config` command. Previous decisions established parameter-driven plugin activation, with FPL activating when the `plugins.fpl` block exists and using its built-in all-list URL when the URL is omitted.

A product boundary was needed for whether the example YAML should start with no active proxy sources, require the user to fill credentials before serving usefully, or enable a no-credential source by default.

## Decision
The v1 example YAML enables the FPL plugin by default.

The example configuration should be directly runnable and should include the `plugins.fpl` block so AIOPROXY has a no-credential proxy source on first run. FOFA and sing-box remain documented as commented or fill-in templates because they require user-specific key, subscription URL, or local file inputs.

If FPL refresh fails or imports zero usable candidates, the service still follows the existing degraded-health and empty-pool fast-failure behavior.

## Consequences
- New users can run the example configuration without FOFA credentials or sing-box subscriptions.
- First startup may perform an external request to the default FPL source.
- The example configuration is more likely to produce candidate proxies out of the box, subject to FPL source quality and validation success.
- Documentation must clearly state that FPL is enabled by the example and that users can remove the `plugins.fpl` block if they do not want startup to access the FPL source.
- FOFA and sing-box setup remain explicit user configuration steps.
