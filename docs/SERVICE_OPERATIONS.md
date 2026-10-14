# Persistent Go service operations

This runbook covers a controlled service-mode rollout and rollback. Completing
these steps in documentation does not back up or change a production system.
Keep the live-account smoke step gated on explicit operator approval.

## Required paths and ownership

Choose one durable data directory and one output directory visible to the Go
service and Python API/MCP processes. Set `FLOW_DATA_DIR` (or `FLOW_DB_PATH`),
`FLOW_COOKIE_DIR`, and `FLOW_OUTPUT_DIR` explicitly. The extension bridge writes
scoped bundles under the cookie directory; treat that directory and its backups
as credentials. Set the same `FLOW_INTERNAL_TOKEN` for the Go service and every
Python/MCP client. Bind the HTTP API to loopback unless an authenticated private
network deployment has been deliberately configured.

Only one process may own the extension bridge and upstream scheduler at a time.
The service also acquires a database instance lock. A second service pointed at
the same database exits instead of becoming another queue owner.

For strict cookie freshness, each account must have a live extension connection
from the browser profile that owns that account's cookies. A persisted bundle
whose profile is offline stays unavailable to the scheduler even if its cached
cookies can bootstrap an engine. When several profiles are used, install/pair
the extension in each profile and confirm `/health` reports the expected
`connected_accounts` and `ready_accounts` before relying on parallel account
capacity.

## Preflight and backup

1. Pick a maintenance window and stop new FastAPI/MCP intake. Record the current
   backend mode and binary version. Confirm which process currently owns the
   bridge and upstream submissions.
2. Drain or reconcile all in-flight jobs. Never resubmit a job whose status is
   `submission_unknown` until its upstream state has been checked.
3. Stop the current owner cleanly and wait for process exit. Copy the whole
   database directory, including any `-wal` or `-shm` files that remain, and the
   complete cookie-bundle directory to a timestamped, access-controlled backup.
   Do not copy only the main SQLite file while the service is running. Record
   hashes and verify the copy can be opened from a separate staging path.
4. Keep the current output directory and API configuration. Use a staging copy
   of production-shaped data for upgrade and restore rehearsal; do not rehearse
   by opening the production database with a new binary.
5. Verify the new Go binary with `flow-windows.exe version` (or the binary for
   the host), and verify that the old bridge/scheduler owner has exited.

Example PowerShell backup after a clean shutdown:

```powershell
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$backup = Join-Path $env:FLOW_BACKUP_ROOT $stamp
New-Item -ItemType Directory -Path (Join-Path $backup 'data') | Out-Null
$dbPath = if ($env:FLOW_DB_PATH) { $env:FLOW_DB_PATH } else { Join-Path $env:FLOW_DATA_DIR 'flow.db' }
Copy-Item -LiteralPath $dbPath -Destination (Join-Path $backup 'data')
foreach ($suffix in @('-wal', '-shm')) {
    $sidecar = "$dbPath$suffix"
    if (Test-Path -LiteralPath $sidecar) {
        Copy-Item -LiteralPath $sidecar -Destination (Join-Path $backup 'data')
    }
}
Copy-Item -LiteralPath $env:FLOW_COOKIE_DIR -Destination (Join-Path $backup 'cookies') -Recurse
Get-ChildItem $backup -Recurse -File | Get-FileHash -Algorithm SHA256 |
    Export-Csv (Join-Path $backup 'sha256.csv') -NoTypeInformation
```

Run the restore rehearsal against a separate staging directory. Preserve the
original production copy unchanged, compare the hashes, start the new binary
against the restored staging copy, and verify that persisted jobs, attempts,
account records and cookie bundle revisions load as expected.

On account bootstrap, the service reconciles history to an explicit bundle ID
only when exactly one active legacy row has the same SAPISID session fingerprint.
It transfers generation/media ownership, anchors, cooldowns and pinned queued
jobs transactionally, then marks the old row parked and superseded. A missing or
ambiguous fingerprint does not move data; review the startup log and reconcile
from a verified backup before attempting any manual merge.

## Start and verify service mode

Set the following in the Go service process. Keep the token out of command
history and logs; use the deployment secret store where available.

```powershell
$env:FLOW_INTERNAL_TOKEN = 'tainvpool_account'
$env:FLOW_DATA_DIR = 'E:\flow\data'
$env:FLOW_COOKIE_DIR = 'E:\flow\cookies'
$env:FLOW_OUTPUT_DIR = 'E:\flow\output'
$env:WS_PORT = '9222'
.\bin\flow-windows.exe service --listen 127.0.0.1:9223 --workers 8
```

After the extension has attached and accounts have bootstrapped, check both
health endpoints:

```powershell
Invoke-RestMethod http://127.0.0.1:9223/health
Invoke-RestMethod http://127.0.0.1:9223/readyz
```

`/health` reports process health. `/readyz` must return HTTP 200 with
`status=ready` and at least one `ready_accounts` before switching API intake.
In a second process, use the same paths and token and set:

```powershell
$env:FLOW_ENGINE_MODE = 'service'
$env:FLOW_SERVICE_URL = 'http://127.0.0.1:9223'
$env:FLOW_INTERNAL_TOKEN = 'tainvpool_account'
$env:FLOW_OUTPUT_DIR = 'E:\flow\output'
```

Start FastAPI/MCP only after `/readyz` succeeds. Verify API health and service
stats with the bearer token, then observe a canary request after operator
approval. Watch queued/running/unknown job counts, queue wait, ready accounts,
cookie sync age, refresh count, auth failures, throttle cooldowns and
`submission_unknown`. Keep only one API execution mode active during the canary.

With the same service-mode environment available to the API process, run the
read-only preflight before reopening intake:

```powershell
python scripts/service_cutover_preflight.py --api-url http://127.0.0.1:8001
```

It checks explicit mode/URL/token configuration, Go health and readiness,
authenticated stats access, live-account requirements for strict cookie mode,
and FastAPI's service-mode health. It never prints the bearer token and does not
submit a generation request or change intake state.

## Rollback

1. Stop new API intake while leaving the Go service alive.
2. Reconcile queued, running, accepted-media and `submission_unknown` jobs. Allow
   active jobs to finish; confirm accepted outputs before deciding whether any
   job is safe to retry.
3. Send Ctrl+C/SIGTERM to the Go service and wait for its bounded drain and
   shutdown to finish. Confirm the process and bridge listener have exited.
4. Start FastAPI/MCP with `FLOW_ENGINE_MODE=subprocess` and remove
   `FLOW_SERVICE_URL`. Confirm the old backend is the only upstream owner before
   reopening intake.
5. Keep the service database, cookie bundles and pre-cutover backup intact for
   reconciliation. Restore the pre-cutover database only if rollback testing
   proves it is required; restoring over newer job state can erase acknowledgments
   and cause duplicate work.

Do not run the real-account canary, stop a production owner, or switch production
intake until the operator has approved the window, verified backups and confirmed
that the single-owner condition is met.
