# ADR 0061: Structured username routing and ip-api country validation

## Status
Accepted

## Context
AIOPROXY v1 already supports explicit session binding through the proxy authentication username. The existing accepted syntax uses the hyphen form `credential-session-ttl`, which works well for session identity but does not provide a clear place to express upstream-routing constraints such as “only use one plugin” or “only use one proxy-exit country”.

A new product boundary was needed for two related capabilities:

1. how plugin- or region-specific routing should be expressed without breaking the accepted legacy session username format; and
2. how region metadata should be obtained in a way that participates in the same update-time validation pipeline used for candidate admission.

The user explicitly required:

- routing by username to one plugin;
- routing by username to one region;
- region mode to be driven by automatic requests during validation rather than by static labels;
- filtered empty sets to fail fast rather than silently falling back to the global pool.

## Decision
AIOPROXY v1 adds a structured username form using `~key=value` segments for routing-aware requests.

The accepted structured keys are:

- `plugin`
- `region`
- `session`
- `ttl`

Examples:

- `aio~plugin=fpl`
- `aio~region=US`
- `aio~plugin=singbox~region=HK~session=job-001~ttl=30m`

The legacy username form without `~` remains fully supported and unchanged.

For region-aware candidate admission, AIOPROXY adds the validation strategy `ip_api_country`. In this mode, update-time validation sends the validation request through the candidate itself to ip-api and only admits the candidate if the response succeeds and includes a non-empty `countryCode`. The returned `countryCode` and `country` become candidate metadata used by region routing and Admin API pool visibility.

## Consequences
- The legacy hyphen session syntax remains backward compatible.
- Structured routing is explicit and does not overload the legacy session parsing rule.
- Plugin routing matches the candidate plugin source name such as `fpl`, `fofa`, or `singbox`.
- Region routing matches the validated candidate metadata field `country_code`, using uppercase country-code semantics.
- When both `plugin` and `region` are present, routing applies both filters and schedules only within the intersection.
- If the filtered candidate set is empty, requests fail fast and do not fall back to the unfiltered pool.
- Session bindings become scope-aware: the same session identifier under different plugin or region constraints does not share one binding.
- Admin `/pool` may expose non-secret country metadata for candidates validated through `ip_api_country`.
- The default validation behavior remains the existing lightweight HTTP status check; `ip_api_country` is opt-in through configuration.
