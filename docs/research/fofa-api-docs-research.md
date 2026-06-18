# FOFA-compatible API docs research

Date: 2026-06-17

## Source

- User-provided docs entry: `https://fofa.icu/docs.php?doc=6e8c196704`
- Query interface page reached from the docs: `https://fofa.icu/docs.php?doc=be5b628f09`
- Account info page reached from the docs: `https://fofa.icu/docs.php?doc=b0a31db88e`
- Continuous pagination page reached from the docs: `https://fofa.icu/docs.php?doc=d8f1eb2a6d`

## Findings

- The docs describe `fofa.icu` as a FOFA relay/proxy platform whose API calling method is compatible with the official FOFA API.
- Search API path: `/api/v1/search/all`.
- Continuous pagination API path: `/api/v1/search/next`.
- Account info API path: `/api/v1/info/my`.
- Search authentication parameter: `key`.
- Search query parameter: `qbase64`, a base64-encoded FOFA query string.
- Search field parameter: `fields`; default is `host,ip,port`.
- Search pagination parameters include `page`, `size`, `full`, and `r_type`.
- The documented max `size` is 10000 per page.
- Supported fields include `ip`, `port`, `protocol`, `host`, `link`, `banner`, `header`, `title`, `base_protocol`, and many metadata fields.
- Response examples show `error`, point usage fields, `size`, `page`, `query`, and `results`, where each result row follows the configured field order.
- The docs mention a daily query limit and error code `-703` for exceeding the daily limit.

## Implication for AIOPROXY

The FOFA plugin should use a configurable `base_url`, `key`, `fields`, `size`, and query list. Since `results` rows follow `fields` order, the plugin must map configured fields deterministically. For proxy candidate extraction, the safest v1 fields are `host,ip,port,protocol` or `ip,port,protocol,host`; each FOFA source query should explicitly declare the intended output proxy protocol instead of relying only on returned `protocol` metadata.
