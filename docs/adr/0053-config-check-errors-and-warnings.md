# ADR 0053: Config check reports errors and warnings separately

## Status
Accepted

## Context
AIOPROXY v1 provides `aioproxy check -c config.yaml` to parse and validate YAML configuration and report which plugins would be active. The command does not bind listeners, start service loops, or refresh proxy sources.

Several accepted product decisions allow service startup in degraded states, such as no active plugins or empty candidate pools. The config check command therefore needs to distinguish hard invalid configuration from valid-but-risky or degraded operation.

## Decision
The config check command reports errors and warnings separately.

Errors represent configuration that prevents valid startup or violates a hard product boundary. The command exits non-zero when errors exist.

Warnings represent configuration that can still start but may be degraded, incomplete, risky, or noteworthy. The command exits zero when warnings exist without errors.

Examples of errors include YAML syntax errors, invalid listener addresses, invalid durations, Admin API non-loopback exposure without token, missing username/password when auth is enabled, and invalid credential/session configuration.

Examples of warnings include no active plugins, plugin blocks missing activation parameters, potentially empty Candidate Pool at startup, debug logging potentially writing secrets to disk, FOFA not activating because the key is empty, and FPL using the built-in default URL.

## Consequences
- CI and deployment scripts can rely on a non-zero exit code for hard configuration failure.
- Operators can see degraded or risky states without being blocked from intentionally starting the service.
- The command output must make plugin activation and warnings clear enough to support pre-deployment review.
- Documentation must explain the difference between check errors and warnings.
