#!/usr/bin/env bash
set -Eeuo pipefail

display_number="${DISPLAY#:}"
display_number="${display_number%%.*}"
socket="/tmp/.X11-unix/X${display_number}"

for _ in $(seq 1 60); do
  [[ -S "$socket" ]] && break
  sleep 0.5
done
if [[ ! -S "$socket" ]]; then
  echo "browser: X display ${DISPLAY} did not become ready" >&2
  exit 1
fi

profiles="${FLOW_BROWSER_PROFILES:-profile-1}"
extension_dir=/opt/flow-extension
state_dir=/var/lib/flow/browser
pids=()

stop_browsers() {
  if ((${#pids[@]})); then
    kill -TERM "${pids[@]}" 2>/dev/null || true
    wait "${pids[@]}" 2>/dev/null || true
  fi
}
trap stop_browsers EXIT INT TERM

IFS=',' read -ra requested <<< "$profiles"
for raw_name in "${requested[@]}"; do
  name="$(printf '%s' "$raw_name" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  if [[ -z "$name" || "$name" == "." || "$name" == ".." || ! "$name" =~ ^[A-Za-z0-9._-]+$ ]]; then
    echo "browser: invalid profile name: ${raw_name}" >&2
    exit 1
  fi

  profile_dir="$state_dir/$name"
  install -d -m 0700 "$profile_dir"
  echo "browser: starting Chromium profile ${name}"
  chromium \
    --user-data-dir="$profile_dir" \
    --load-extension="$extension_dir" \
    --no-sandbox \
    --no-first-run \
    --no-default-browser-check \
    --password-store=basic \
    --disable-features=Translate \
    --window-size=1100,700 \
    https://flow.google.com/ &
  pids+=("$!")
  sleep 1
done

if ((${#pids[@]} == 0)); then
  echo "browser: no profiles configured" >&2
  exit 1
fi

# Restart all profiles if any Chromium process exits, so an account is not lost
# silently while the launcher still appears healthy.
set +e
wait -n "${pids[@]}"
exit_code=$?
set -e
echo "browser: a Chromium profile exited with status ${exit_code}; restarting the group" >&2
exit "$exit_code"

