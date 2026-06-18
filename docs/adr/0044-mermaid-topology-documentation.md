# ADR 0044: Use Mermaid-first topology documentation

## Status
Accepted

## Context
AIOPROXY v1 requires a complete, fine-grained project topology. The topology must stay maintainable as the implementation evolves and should be readable directly in the repository.

## Decision
AIOPROXY v1 documents topology primarily with Mermaid diagrams plus concise layered explanations.

The topology documentation should live under the documentation library and include multiple focused diagrams, such as overall architecture, request flow, plugin refresh and validation flow, session binding, sing-box node bridging, persistence and snapshots, and release build flow.

## Consequences
- GitHub can render the diagrams directly from Markdown.
- The topology remains version-controlled and easy to update alongside code.
- No extra image generation pipeline is required for v1.
- ASCII duplicate diagrams and generated PNG/SVG assets are out of scope unless a future requirement changes this.
