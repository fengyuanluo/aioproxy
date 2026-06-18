# ADR 0057: Example config enables proxy authentication with sample credentials

## Status
Accepted

## Context
AIOPROXY v1 supports username/password authentication for the External Proxy Service and defaults to authentication-enabled operation. v1 supports a single static credential, and explicit session identifiers are encoded in the proxy authentication username. The example YAML should be runnable for quickstart use while still nudging safe operation when the proxy is exposed beyond loopback.

A product boundary was needed for whether the example should disable auth, require the user to fill credentials before startup, or provide sample credentials.

## Decision
The v1 example YAML enables External Proxy Service authentication by default and provides low-risk sample credentials.

The default example credential is intended for local quickstart use, such as username `aio` and password `change-me`. Documentation must strongly instruct users to change the password before exposing the proxy listener to non-local clients.

## Consequences
- The quickstart can demonstrate authenticated HTTP and SOCKS5 proxy usage immediately.
- Session username examples can be based on the sample credential, such as `aio-session1` or `aio-job-001-30m`.
- The runnable example remains aligned with the default-auth product posture.
- Exposing the proxy beyond loopback with unchanged sample credentials is unsafe and must be clearly warned against in documentation.
