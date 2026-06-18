# ADR 0056: Default proxy listener uses loopback

## Status
Accepted

## Context
AIOPROXY exposes a mixed HTTP/SOCKS5 External Proxy Service. The product is intended for single-instance self-use or a small trusted client set, and it supports username/password authentication that can be disabled. The example YAML should be runnable while still using safe defaults.

A product boundary was needed for whether the default proxy listener should bind only to the local host, bind to all interfaces, or require the user to fill a listener address before startup.

## Decision
The default External Proxy Service listener is `127.0.0.1:1080`.

Users who want to serve other machines must explicitly change the listener address, such as to `0.0.0.0:1080` or a specific LAN address.

## Consequences
- The runnable example does not accidentally expose an open proxy to the network.
- The default aligns with the Admin API's local-first security posture.
- Quickstart commands can test the proxy locally without additional network configuration.
- Documentation must explain how to intentionally expose the proxy to trusted clients and the risks of doing so.
