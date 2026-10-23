# Runtime settings in the admin dashboard

Updated: 2026-10-23

## Objective

Expose service settings that can be changed safely while the Go service is
running. Persist them in the existing SQLite `settings` table, apply changes to
the live service, and add a **C蘯･u hﾃｬnh** page to the FastAPI admin dashboard.
Do not add a new authentication flow; use the existing service/API request
paths.

## Findings and setting inventory

`--workers` is not the durable queue worker count. It currently sets the Go
service's global submission gate and account bootstrap parallelism. Unless
`FLOW_QUEUE_WORKERS` is set, queue worker goroutines default to
`max(16, --workers * 4)`, capped at 256. Account concurrency is separately
hard-coded to one in both the account runtime gate and scheduler worker.

| Admin setting | Current source | Runtime change | Notes |
| --- | --- | --- | --- |
| `submission_concurrency` | `--workers`; `AccountManager.global` | Yes | Controls concurrent generation submissions. Account bootstrap parallelism remains a startup-only use of `--workers`. |
| `queue_workers` | `FLOW_QUEUE_WORKERS` or derived default | Yes | Requires a worker supervisor that can grow or shrink without abandoning leased jobs. |
| `account_concurrency` | hard-coded `1` in account gate and scheduler worker | Yes | Both limits must change together. Lowering the limit must let active jobs finish. |
| `media_poll_concurrency` | `FLOW_MEDIA_POLL_CONCURRENCY` | Yes | Replace fixed channel capacity with an adjustable limiter shared by engines. |
| `media_download_concurrency` | `FLOW_MEDIA_DOWNLOAD_CONCURRENCY` | Yes | Same adjustable limiter; active downloads finish under the old limit. |
| `max_outstanding_jobs` | `FLOW_MAX_OUTSTANDING_JOBS` | Yes | Store currently describes this as startup-only and its in-memory field needs synchronized access. |
| `max_job_attempts` | `FLOW_JOB_MAX_ATTEMPTS` | Yes | Apply to future retry decisions; do not rewrite existing attempt history. |
| retry budgets | `FLOW_JOB_MAX_PRE_SUBMIT_ATTEMPTS`, `FLOW_JOB_MAX_RESUME_ATTEMPTS`, `FLOW_JOB_MAX_THROTTLE_ATTEMPTS` | Yes | Apply to future retry decisions; keep the current safe-retry rules. |
| `job_retention_days` | `FLOW_JOB_RETENTION_DAYS` | Yes | Cleanup loop currently captures its startup value; read the live setting and run cleanup after an update. |
| `cookie_freshness` | `FLOW_COOKIE_FRESHNESS` | Yes | Strict/cached policy can change for new scheduling decisions; keep the final pre-execution connection check. |

Process paths and ownership settings such as `FLOW_DATA_DIR`, `FLOW_DB_PATH`,
`FLOW_COOKIE_DIR`, `FLOW_OUTPUT_DIR`, listener addresses, service URLs, proxy,
and credentials remain startup-only. They affect open files, listeners, or
client construction and should not be edited from this page. `FLOW_SERVICE_DRAIN_SECONDS`
only affects shutdown and is outside the runtime settings page.

## Runtime behavior

- Stored admin values override startup defaults after service initialization.
  If a key has never been saved, retain the current CLI/environment-derived
  default. Resetting a field removes its stored override and restores that
  default immediately.
- Validate the entire update before persisting it. Persist all changed keys in
  one SQLite transaction, then apply the already validated update to memory.
- Increasing a limit admits more work after save. Decreasing it does not cancel
  active jobs; it prevents new work from exceeding the new limit as slots free.
- Lowering `queue_workers` retires workers after their current job. A worker
  must not abandon a durable lease just because the target count decreased.
- Settings responses report effective values and their source (`admin` or
  `startup default`), so the dashboard shows what the service is using.

## Implementation tasks

### T1. Define and persist runtime settings

- [x] Add a typed settings model with defaults, bounds, names, and validation
      for every row in the inventory.
- [x] Add transactional multi-key save and reset operations using the existing
      SQLite settings table.
- [x] Load persisted overrides during service startup after CLI/environment
      defaults are known.
- [x] Keep the existing environment variables as fallback values for
      installations without saved admin settings.

### T2. Add adjustable runtime limits

- [x] Add a resizable limiter whose current occupancy is preserved when its
      limit changes; use it for global submissions, account gates, media polls,
      and media downloads.
- [x] Update scheduler worker concurrency through a synchronized setter and
      apply the same account limit to each account's execution gate.
- [x] Add a queue worker supervisor that starts workers when the target grows
      and lets idle workers retire after finishing their current lease when it
      shrinks.
- [x] Make outstanding-job admission reads and writes concurrency-safe.
- [x] Protect retry budgets and retention values for concurrent reads during
      live updates.
- [x] Make cookie freshness updates visible to eligibility checks and wake
      waiting scheduler requests after a policy change.

### T3. Expose the Go settings API

- [x] Add a settings read endpoint returning effective values, defaults,
      sources, and live in-flight counts where applicable.
- [x] Add a partial update endpoint with strict allowlisted keys, numeric
      bounds, enum validation, and transactional persistence.
- [x] Add a reset-to-startup-default operation for selected keys.
- [x] Apply an update only after persistence succeeds; return field-level
      validation errors without partially changing runtime state.

### T4. Connect FastAPI and the dashboard

- [x] Add FastAPI proxy methods and endpoints for reading, updating, and
      resetting Go service settings.
- [x] Add a **Cấu hình** sidebar menu and page with editable controls for
      concurrency, queue/retry policy, retention, and cookie freshness.
- [x] Show effective value, startup fallback, validation bounds, and whether a
      decrease is waiting for active work to finish.
- [x] Add save, per-field reset, success, and error states; refresh effective
      values after each operation.
- [x] Do not add a new authentication flow.

### T5. Verify behavior and document operations

- [ ] Verify settings survive a Go service restart and environment values remain
      fallbacks when no admin override exists.
- [ ] Verify increasing and decreasing global, queue, per-account, and media
      concurrency while work is active; confirm active work is not cancelled.
- [ ] Verify account isolation, cookie rotation, scheduler availability, safe
      retry behavior, queue capacity, and retention after live updates.
- [ ] Verify malformed, out-of-range, and unknown settings are rejected without
      partial writes.
- [x] Document the settings, bounds, default precedence, and active-job behavior
      in the service operations guide.

## Acceptance criteria

- Operators can change the listed runtime settings from the admin page without
  restarting the Go service.
- The dashboard and Go API show the same effective values after save and after
  restart.
- Existing jobs continue safely when a limit is reduced; new work follows the
  new limit as capacity becomes available.
- Account concurrency changes affect both scheduler admission and runtime
  execution gates, without crossing account/session boundaries.
- Startup-only path, listener, and credential settings are not presented as
  live-editable settings.
