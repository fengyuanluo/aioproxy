# ADR 0030: Activate plugins based on configured parameters

## Status
Accepted

## Context
AIOPROXY has multiple plugins with different configuration requirements. The user prefers product behavior where a plugin starts when its required parameters are configured and stays inactive when those parameters are left empty.

## Decision
Plugin activation is parameter-driven rather than controlled primarily by an `enabled` flag. FOFA starts when its required parameters such as base URL/key are configured. sing-box starts when subscription/config sources are configured. Plugins with empty required parameters stay inactive.

## Consequences
- YAML stays concise and declarative.
- Missing optional plugin configuration does not create degraded startup noise.
- Plugins without required parameters need an explicit product default for whether they run automatically.
