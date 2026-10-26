#!/usr/bin/env bash
set -Eeuo pipefail

state_dir=/var/lib/flow
token_file="$state_dir/internal-token"

install -d -m 0750 \
  "$state_dir/data" \
  "$state_dir/cookies" \
  "$state_dir/output" \
  "$state_dir/browser"

chown -R flow:flow "$state_dir"

if [[ -z "${FLOW_INTERNAL_TOKEN:-}" ]]; then
  if [[ -s "$token_file" ]]; then
    FLOW_INTERNAL_TOKEN="$(<"$token_file")"
  else
    FLOW_INTERNAL_TOKEN="$(python3 -c 'import secrets; print(secrets.token_urlsafe(48))')"
    umask 077
    printf '%s\n' "$FLOW_INTERNAL_TOKEN" > "$token_file"
    chown flow:flow "$token_file"
  fi
fi
export FLOW_INTERNAL_TOKEN

exec /usr/bin/supervisord -n -c /etc/supervisor/supervisord.conf
