# ADR 0015: FOFA v1 exposes only single-request fetch size

## Status
Accepted

## Context
FOFA-compatible search APIs support pagination through `page`, `size`, and continuous pagination endpoints. AIOPROXY v1 needs a simple and predictable FOFA collection model.

## Decision
FOFA v1 exposes only the single-request fetch amount through YAML. Each configured FOFA query performs one search request with the configured `size`. Pagination controls such as page start, page limit, and continuous `/search/next` are not part of v1 configuration. The request page is fixed to the first page.

## Consequences
- FOFA point and network cost are predictable per update.
- Configuration stays simple.
- Operators can increase or decrease one fetch amount but cannot request multiple pages in v1.
- More advanced FOFA pagination can be added later if needed.
