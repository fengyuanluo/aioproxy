# ADR 0043: Treat 300 concurrent clients as a v1 acceptance gate

## Status
Accepted

## Context
AIOPROXY is expected to be a high-performance proxy aggregator rather than a proof-of-concept. The original product requirement states that v1 must support at least 300 concurrent clients without crashing.

A measurable completion standard is needed so the performance requirement cannot degrade into a design-only claim.

## Decision
AIOPROXY v1 is not considered complete unless it passes an automated local stress test with at least 300 concurrent clients.

The acceptance evidence must show that during the stress scenario the process does not panic, crash, deadlock, or become unable to serve the core proxy path. The final delivery report must include the stress command and decisive output.

## Consequences
- High-concurrency behavior becomes a release-blocking acceptance criterion.
- The project must include repeatable stress-test tooling or scripts sufficient to reproduce the 300-concurrent-client check.
- Changes affecting the proxy hot path must be revalidated against this gate before being treated as complete.
- The final report must distinguish passing stress evidence from ordinary build or unit-test success.
