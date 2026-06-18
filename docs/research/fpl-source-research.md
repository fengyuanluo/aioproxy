# proxifly/free-proxy-list source research

Date: 2026-06-17

## Source checked

- Repository: `https://github.com/proxifly/free-proxy-list`
- Default all-list raw/CDN URL checked: `https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/all/data.txt`

## Observed all-list format

The checked all-list URL returned HTTP 200 with plain text proxy URLs, one per line.

Observed counts at check time:

- total non-empty lines: 3396
- `http`: 2241
- `socks4`: 758
- `socks5`: 397

## Implication for AIOPROXY

The FPL plugin can use the all-list URL as a single default source. It should parse scheme-prefixed proxy URLs, import only `http` and `socks5`, skip `socks4`, and rely on update-time validation before pool admission.
