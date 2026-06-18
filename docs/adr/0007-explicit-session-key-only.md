# ADR 0007: Session binding requires an explicit session identifier

## Status
Accepted

## Context
AIOPROXY supports authenticated external proxy access and multiple scheduling modes, including random, round-robin, and session binding. Authentication identity and session identity are separate concepts.

## Decision
Session binding only applies when the client provides an explicit session identifier. The client credential username must not implicitly become the session key. Authenticated traffic without an explicit session uses the configured non-session scheduling mode, such as random or round-robin.

HTTP clients provide the explicit session identifier through a configured HTTP header. SOCKS5 clients provide it through a configured username encoding rule that separates the credential username from the session identifier while still authenticating the credential username.

## Consequences
- A single credential can run many independent sessions.
- Normal authenticated traffic is not accidentally pinned to one upstream proxy.
- Client documentation must specify how to pass explicit session identifiers for HTTP and SOCKS5.
