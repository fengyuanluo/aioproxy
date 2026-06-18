# ADR 0011: Use one optional static entry credential in v1

## Status
Accepted

## Context
AIOPROXY v1 is a single-instance proxy aggregator for a trusted operator or small trusted client set. It supports username/password authentication and explicit session binding encoded in the proxy username.

## Decision
Entry authentication is enabled by default but can be disabled. When enabled, v1 supports exactly one static credential username and password. Multiple credential usernames are not supported in v1. When authentication is disabled, requests are anonymous, explicit session binding is unavailable, and traffic uses the configured non-session scheduling policy.

## Consequences
- Authentication remains simple and aligned with the single-instance v1 scope.
- Session username parsing can assume a single credential username prefix.
- Multi-user, per-user policy, quota, and credential rotation are out of v1 scope.
- Disabling authentication removes the username channel used for explicit sessions.
