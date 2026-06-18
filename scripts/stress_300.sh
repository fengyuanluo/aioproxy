#!/usr/bin/env bash
set -euo pipefail
proxy_url=${1:-http://aio:change-me@127.0.0.1:1080}
target_url=${2:-http://example.com/}
seq 1 300 | xargs -n1 -P300 -I{} curl -fsS -o /dev/null -x "$proxy_url" "$target_url"
echo "stress_300: ok"
