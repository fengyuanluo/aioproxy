# ADR 0008: Use random as the default non-session scheduling policy

## Status
Accepted

## Context
AIOPROXY supports ordinary non-session traffic and explicit session-bound traffic. Session binding is not a default scheduling policy; it is activated only when a request carries an explicit session identifier.

## Decision
Non-session requests use the configured non-session scheduling policy. The default non-session scheduling policy is random. YAML configuration must allow changing it to round-robin.

## Consequences
- Authenticated requests without an explicit session identifier are not pinned by username.
- The default behavior spreads traffic randomly across the candidate pool.
- Operators can switch to round-robin when deterministic distribution is preferred.
