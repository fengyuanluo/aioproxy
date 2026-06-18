# sing-box in-process per-node dialer research

Date: 2026-06-17

## Sources checked

- Official repository: https://github.com/SagerNet/sing-box
- Latest stable release observed from GitHub API: `v1.13.13`, published 2026-06-04.
- Current default branch observed from GitHub API: `testing`.
- Local module cache inspected: `/home/go/pkg/mod/github.com/sagernet/sing-box@v1.13.13`.
- Current testing branch source inspected from downloaded zip: `/tmp/singbox-src/sing-box-testing`.

## Findings

1. `adapter.Outbound` embeds `github.com/sagernet/sing/common/network.Dialer`, so every sing-box outbound is already a dialer surface from the library perspective.
2. `adapter.OutboundManager` exposes `Outbound(tag string) (Outbound, bool)` and `Outbounds() []Outbound`, so a caller can retrieve a specific outbound by tag.
3. `box.Box` exposes `Outbound() adapter.OutboundManager` in both stable `v1.13.13` and current `testing`, so an embedded `Box` can provide per-tag outbound access.
4. The default include registry registers HTTP, SOCKS, Shadowsocks, VMess, Trojan, VLESS, AnyTLS, group outbounds, and QUIC-backed outbounds when the corresponding build tags are enabled.
5. Stable `v1.13.13` and current `testing` both support the core shape needed for a per-node in-process dialer: build a sing-box config with one outbound per subscription node, start a `Box`, retrieve an outbound by tag, and call `DialContext` on that outbound.
6. The sing-box repository itself does not appear to provide a broad generic subscription/share-link conversion layer in the core repository. It primarily consumes sing-box JSON configuration objects. Subscription formats and share links likely require AIOPROXY-side parsing/conversion or an additional converter dependency.

## Preliminary conclusion

Per-node in-process dialer is feasible at the sing-box core API level. The main unresolved risk is not whether an outbound can dial; it is how AIOPROXY will convert subscription/share-link inputs into sing-box `option.Outbound` entries and which sing-box protocol/build-tag set v1 will officially support.

## Subscription samples checked

User-provided samples:

- `http://my.599520.xyz:8199/all.yaml`
- `http://my.599520.xyz:8199/base64.txt`

DNS/cache note: the host resolved only to IPv6 (`2409:8a50:542b:d8e0::1000`) in this environment. `curl -4` failed. Fetching by IPv6 literal with `Host: my.599520.xyz:8199` succeeded.

Observed sample structure, redacted:

- `all.yaml`: Clash-like YAML with only top-level `proxies`; 1115 proxy entries.
- `all.yaml` proxy type counts: `ss=785`, `vless=149`, `http=100`, `hysteria2=39`, `vmess=23`, `trojan=16`, `socks5=1`, `mieru=1`, `hysteria=1`.
- `base64.txt`: base64-encoded share-link list; 1014 decoded non-empty lines.
- `base64.txt` scheme counts: `ss=785`, `vless=149`, `hysteria2=39`, `vmess=23`, `trojan=16`, `socks=1`, `hysteria=1`.

Implication: v1 option C requires both Clash-like YAML proxy conversion and base64/share-link conversion. It also requires an unsupported-type policy for types such as `mieru` unless an additional implementation dependency is accepted.
