# ADR 0033: Use a split documentation library instead of a monolithic README

## Status
Accepted

## Context
AIOPROXY v1 has multiple product concepts: mixed proxy entry, explicit session username expressions, plugin activation, source refresh schedules, snapshots, persistence, admin status, and release/deployment guidance. A single README would become too large.

## Decision
AIOPROXY uses a split documentation library. The README stays concise and points to focused documents under `docs/`. Product documentation is organized by topic instead of being packed into one long README.

## Consequences
- Users can navigate to the topic they need.
- README remains suitable for quick orientation.
- Documentation maintenance requires keeping multiple focused files in sync.
