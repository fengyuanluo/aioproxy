# ADR 0041: Use stage-based Git commits

## Status
Accepted

## Context
AIOPROXY is developed under Git version control. The project includes product clarification records, architecture documentation, service implementation, plugins, admin surfaces, persistence, logging, and release automation.

## Decision
AIOPROXY development uses stage-based commits rather than one large final commit. Logical stages such as documentation decisions, project skeleton, core proxy service, pool scheduling, plugins, admin/persistence/logging, and release documentation should be committed separately when each stage is validated.

## Consequences
- History remains reviewable and easier to revert.
- Each stage can include its own validation evidence.
- Commits should not be made before the relevant stage is actually complete.
