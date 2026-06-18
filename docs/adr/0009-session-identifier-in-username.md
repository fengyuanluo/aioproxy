# ADR 0009: Encode explicit session identifiers in the proxy username

## Status
Accepted

## Context
AIOPROXY supports explicit session binding for HTTP and SOCKS5 proxy clients. Session binding is activated through the proxy authentication username field rather than a separate scheduling policy.

## Decision
An explicit session identifier is encoded in the proxy username using the pattern `username-sessionid-{optional session lifetime}`. Requests whose username does not include an explicit session identifier are non-session requests and use the configured non-session scheduling policy.

## Consequences
- HTTP and SOCKS5 clients can use the same session expression mechanism through proxy authentication.
- A single credential username can carry multiple session identifiers.
- The exact parsing rule for hyphen-containing usernames, session IDs, and optional lifetime must be specified before implementation.

## Parsing Rule

The accepted v1 parsing rule is:

- `credential` must not contain `-`.
- The first `-` separates the credential from the session expression.
- If the final session expression segment is a valid duration, it is treated as the requested session lifetime.
- The remaining session expression is the session identifier and may contain `-`.
- Password validation applies only to `credential`.
- The session identifier and requested lifetime do not participate in authentication.

Examples:

- `aio` means credential `aio`, no session.
- `aio-job001` means credential `aio`, session `job001`.
- `aio-job-001` means credential `aio`, session `job-001`.
- `aio-job-001-30m` means credential `aio`, session `job-001`, requested lifetime `30m`.
