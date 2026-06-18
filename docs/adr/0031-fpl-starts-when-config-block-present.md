# ADR 0031: Start FPL only when its config block is present

## Status
Accepted

## Context
Plugin activation is parameter-driven. FPL has a built-in default all-list URL and does not require credentials or subscription parameters.

## Decision
The FPL plugin starts only when the YAML configuration contains a `plugins.fpl` block. The FPL URL may be omitted inside that block; when omitted, AIOPROXY uses the built-in all-list URL.

## Consequences
- FPL does not run implicitly when the operator leaves plugin configuration empty.
- Operators can enable FPL with a minimal config block.
- The built-in FPL source remains available without requiring users to copy the URL.
