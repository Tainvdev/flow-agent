#!/usr/bin/env bash
set -Eeuo pipefail

port="${FLOW_NOVNC_PORT:-6080}"

for _ in $(seq 1 60); do
  if bash -c 'exec 3<>/dev/tcp/127.0.0.1/5900' 2>/dev/null; then
    exec websockify --web=/usr/share/novnc/ "$port" 127.0.0.1:5900
  fi
  sleep 0.5
done

echo "novnc: x11vnc did not become ready" >&2
exit 1

