# ADR 0010: Use sliding session TTL with max TTL clamping

## Status
Accepted

## Context
AIOPROXY supports explicit session binding through proxy usernames with an optional requested session lifetime. Session lifetime must support long-running jobs without leaking stale sessions indefinitely.

## Decision
Session TTL is sliding: each use refreshes the session expiration. Requests without an explicit TTL use the configured default session TTL. Requests with a TTL greater than the configured maximum are clamped to the maximum rather than rejected. If a session-bound candidate is evicted at runtime, the session is rebound to another available candidate on the next use.

## Consequences
- Active jobs keep their proxy binding as long as they continue using the session.
- Over-large TTL requests remain user-friendly through clamping.
- Stale sessions expire after inactivity.
- Session binding depends on candidate availability; evicted candidates force rebinding.
