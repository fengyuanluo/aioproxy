# ADR 0055: Provide a minimal quickstart document

## Status
Accepted

## Context
AIOPROXY v1 is released as multi-platform binaries and uses YAML configuration only. It has a concise README, a split Chinese-first documentation library, a fully commented example YAML, GitHub Actions binary releases, and a systemd example. The example YAML enables FPL by default so new users can run the product without FOFA credentials or sing-box subscriptions.

A product boundary was needed for whether first-run instructions should live only in README, only in full configuration documentation, or in a dedicated quickstart document.

## Decision
AIOPROXY v1 includes a dedicated `docs/quickstart.md` document for the shortest runnable path.

The quickstart should cover downloading or using the binary, copying the example YAML, running `aioproxy check -c config.yaml`, running `aioproxy serve -c config.yaml`, testing HTTP proxy usage, testing SOCKS5 proxy usage, checking Admin API health, and understanding common degraded-health causes.

The quickstart must also call out default credentials, the fact that the example configuration enables FPL and may access the external FPL source on startup, and the expected behavior when the Candidate Pool is empty.

## Consequences
- New users have a short path from binary to a running service.
- README can remain concise and link to the quickstart instead of becoming a full tutorial.
- Full configuration documentation can focus on parameter reference rather than first-run guidance.
- Quickstart examples must stay aligned with the example YAML, default credentials, Admin API behavior, and proxy usage documentation.
