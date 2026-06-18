# ADR 0016: FPL plugin uses the all proxy list with minimal configuration

## Status
Accepted

## Context
AIOPROXY v1 includes an FPL plugin that imports proxies from `proxifly/free-proxy-list`. The AIOPROXY core supports HTTP CONNECT and SOCKS5 upstream proxies, not SOCKS4.

## Decision
The FPL plugin uses the upstream all-proxy list by default and filters it locally. HTTP entries are imported as HTTP proxy candidates, SOCKS5 entries are imported as SOCKS5 proxy candidates, and SOCKS4 entries are skipped with import-report accounting. The default source URL is built in and can be overridden only when needed.

## Consequences
- FPL works with no required plugin-specific configuration beyond enabling the plugin.
- The plugin remains resilient to protocol mix in the all list.
- SOCKS4 support remains out of v1 scope.
- Update-time validation still decides which imported candidates enter the candidate pool.
