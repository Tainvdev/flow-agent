#!/usr/bin/env bash
set -Eeuo pipefail

api_port="${FLOW_API_PORT:-8001}"
curl --fail --silent --show-error --max-time 3 "http://127.0.0.1:9223/health" >/dev/null
curl --fail --silent --show-error --max-time 3 "http://127.0.0.1:${api_port}/health" >/dev/null
