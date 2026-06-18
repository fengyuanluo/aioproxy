# ADR 0013: FOFA plugin uses default and custom source queries

## Status
Accepted

## Context
AIOPROXY v1 includes a FOFA plugin that collects HTTP and SOCKS5 proxy candidates. The user requires configurable FOFA base URL support. The FOFA-compatible documentation at `https://fofa.icu/docs.php?doc=6e8c196704` describes a relay platform whose API paths and calling style are compatible with FOFA official APIs.

## Decision
The FOFA plugin supports built-in default HTTP and SOCKS5 source queries and allows YAML to disable defaults and add custom queries. Each query explicitly declares the output proxy protocol. The plugin consumes FOFA-compatible search APIs using a configurable base URL and key.

## Consequences
- The plugin can work out of the box with default HTTP/SOCKS5 searches.
- Operators can override or extend source queries without changing code.
- Query outputs remain normalized as proxy candidates; core validation decides pool admission.
- Implementation must verify FOFA response field mapping against the configured `fields` order.
