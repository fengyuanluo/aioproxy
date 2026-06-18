# ADR 0014: FOFA plugin ships concrete built-in default query strings

## Status
Accepted

## Context
The FOFA plugin must support built-in default searches for HTTP and SOCKS5 proxy candidates. FOFA query results are still validated by AIOPROXY before entering the candidate pool.

## Decision
The FOFA plugin ships concrete built-in default query strings. For SOCKS5/SK proxy discovery, the default query is `protocol=="socks5" && banner="Method:No Authentication"` as provided by the user. Default query strings may still be overridden through YAML custom query configuration.

## Consequences
- FOFA can work with minimal configuration beyond base URL and key.
- The SOCKS5 default search is based on a concrete protocol and no-authentication banner signal.
- The HTTP default query uses a combined set of proxy-related banner/header/software signals.
- FOFA single-request fetch size must be configurable.
- FOFA results are not automatically trusted; update-time validation still controls pool admission.


## Default Query Strings

SOCKS5/SK default query:

```text
protocol=="socks5" && banner="Method:No Authentication"
```

HTTP default query:

```text
banner="Proxy-Authenticate" || banner="Proxy Authentication Required" || banner="Proxy-Agent" || banner="Squid" || banner="tinyproxy" || banner="3proxy"
```

Default fields:

```text
ip,port,protocol,host
```

The FOFA `size`/single-request fetch amount is configurable through YAML.
