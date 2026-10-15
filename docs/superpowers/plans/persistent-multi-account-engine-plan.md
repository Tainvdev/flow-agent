# Persistent multi-account Go service migration plan

Updated: 2026-10-14

## Objective

Run one long-lived Go service that owns the durable job queue, scheduler, account runtimes, extension bridge, and upstream Flow submissions. Give every account an isolated engine/session, persist and atomically apply cookie rotations, and route FastAPI/MCP requests through the service. The request path must remain safe under concurrent callers, process restarts, request timeouts, and ambiguous upstream submissions.

## Target request flow

1. FastAPI/MCP validates the request and sends one idempotent job submission to the local Go service.
2. Go durably accepts the job and returns its job ID. The Python caller polls, streams events, or waits using that ID; disconnecting does not implicitly cancel accepted work.
3. The shared scheduler selects an eligible account or honors an explicit account pin. It enforces per-account and global limits, cooldowns, credit admission, and known media ownership.
4. That account's runtime synchronizes the latest extension cookie bundle before execution. A rotation replaces only that account's runtime; the per-account execution gate remains held across replacement.
5. Go records phase transitions, upstream media IDs, results and failures in the shared store. It resumes accepted media after restart and never blindly resubmits an ambiguous generation request.

## Decisions and invariants

- The persistent Go service is the sole queue, scheduling, bridge and upstream-submission owner in service mode. Python subprocess mode remains an explicit compatibility mode; configuration errors do not silently fall back.
- Keep one isolated engine, cookie jar, project/auth context and scheduler worker per account. Never share mutable session state across accounts.
- Cookie bundles are scoped to a stable account ID, persisted atomically and reloaded from the newest revision. A stale bootstrap must not publish over a newer rotation.
- A job pinned to an account must wait or fail if that account is unavailable; it must not silently switch accounts. Automatic jobs may choose another eligible account when replay is safe and inputs are portable or re-uploadable.
- Retry only when the durable phase or a typed error proves replay safe. An ambiguous post-submit outcome becomes `submission_unknown`. Account throttles may cool down and fail over within a bounded retry budget. Policy/model refusals must not be retried on another account.
- Enforce bounded queue, global generation submissions, one active generation per account, browser operations, media polling and media downloads. Release the global submission permit after accepted media is durably recorded; retain the account job slot until terminal completion.
- Bind known media IDs to account/project ownership. Do not reuse account-owned assets under another runtime.
- Keep the internal listener on loopback by default and require bearer authentication.

## Work plan and status

### Phase 0: contracts and baseline

- [x] Preserve existing synchronous image/video/chat response shapes and expose native asynchronous job APIs separately.
- [x] Add idempotency keys, durable acknowledgement lookup, request deadlines and explicit cancellation semantics.
- [x] Record cold/warm and one-account/multi-account latency and failure-rate baselines on a controlled workload. The Phase 5 table below records image/video p50/p95 for one and five fake accounts; each row covers 640 accepted jobs and 0% simulated failures.

### Phase 1: isolated account runtime and cookie lifecycle

- [x] Give each registered account its own engine, cookie jar, project/auth context, worker and scheduler state.
- [x] Scope extension refresh/rotation to an account; persist bundles atomically and publish only the newest revision.
- [x] Keep an account execution gate stable while replacing its runtime, so old and new cookie snapshots cannot execute concurrently.
- [x] Require live cookie sync by default; make cached-cookie operation an explicit configuration choice.
- [x] Fail closed on account identity mismatch, missing required auth cookies and failed session refresh; mark only the affected account `needs_login`.

### Phase 2: shared durable queue and scheduler

- [x] Persist jobs, leases, attempts, phase events, upstream IDs, idempotency keys and account cooldowns in SQLite.
- [x] Recover queued jobs and accepted media on startup; mark post-submit expired leases `submission_unknown` when acceptance cannot be determined.
- [x] Enforce configurable queue/worker/submission/browser/media limits, per-account concurrency, cancellation and graceful drain.
- [x] Wake account-scheduler waiters when a slot is released or account availability changes; wait directly to the next cooldown deadline instead of polling every 200 ms. One release wakes one waiter to avoid a thundering herd.
- [x] Select eligible accounts by load/fairness, honoring account pinning, capabilities, credits, cooldowns and asset ownership.
- [x] Expand all-account and multi-image requests into durable parent/child jobs with per-child results and partial failures.
- [x] Gate polling and downloading separately from generation submission; release poll permits between intervals.
- [x] Persist and enforce bounded account-throttle and accepted-media-resume retry budgets.
- [x] Classify `PUBLIC_ERROR_MODEL_ACCESS_DENIED` as a terminal upstream refusal before captcha refresh/retry; preserve its typed error for the durable queue, which does not replay it across accounts.
- [x] Audit retry/error behavior across service upstream paths: only explicit HTTP 429 generation RPCs are safe for bounded account-throttle replay; HTTP 429 elsewhere cools the account without automatic replay; generation and image/video upload transport/5xx ambiguity is terminal; ambiguous resumable video uploads are terminal; accepted-media failures resume durable IDs instead of re-running generation; other untyped resolve/download failures do not authorize generation replay.

### Phase 3: persistent Go service and internal API

- [x] Add `flow service`, one-time store/bridge/registry/scheduler startup and a loopback HTTP listener.
- [x] Add authenticated job submit/status/by-key/cancel, account/project/balance, health/readiness, latency stats and sequenced SSE event endpoints.
- [x] Add single-instance protection for the database and reject oversized payloads or full queues.
- [x] Wake idle queue workers as soon as a new durable job is accepted; retain periodic polling as a recovery backstop and wake workers when children or safe retries are queued.
- [x] Return newly inserted durable jobs from the atomic `INSERT ... RETURNING` statement, avoiding a second serialized SQLite query on the normal acceptance path while preserving idempotent lookup and queue-capacity checks.
- [x] Drain claims before shutdown and recover safe pre-submit or accepted-media attempts after restart.
- [x] Route Go CLI generation commands through the service when service mode is configured.

### Phase 4: FastAPI and MCP migration

- [x] Add a long-lived asynchronous service client managed by FastAPI lifespan and MCP lifecycle.
- [x] Submit each generation once with an idempotency key; poll or stream status without spawning one Go process per request.
- [x] Preserve synchronous compatibility endpoints and response shapes; expose native 202 job submit/status/cancel/SSE APIs.
- [x] Map validation, queue-full, unavailable-account, upstream, and deadline errors to stable HTTP responses.
- [x] Route projects, balances, stats and media operations through the selected service backend.
- [x] Require explicit `FLOW_ENGINE_MODE=service|subprocess`; reject conflicting or incomplete service configuration without silent fallback.
- [x] Update documentation, configuration examples and adapter contract tests.

### Phase 5: correctness, concurrency and resilience verification

- [x] Exercise two isolated account engines against a fake upstream and assert cookie/project/auth context stays paired.
- [x] Exercise 100 concurrent jobs on five fake accounts; assert all settle, all accounts receive work, each account stays at concurrency one and accounts progress in parallel.
- [x] Exercise 401 refresh failure and a 24-job burst; assert one failed account becomes `needs_login`, no retry storm occurs, and a healthy neighboring account continues.
- [x] Exercise a `PUBLIC_ERROR_MODEL_ACCESS_DENIED` response at the durable queue boundary; assert the refusal is terminal after one attempt and is not automatically replayed.
- [x] Set a five-second fallback poll interval and verify a newly accepted job wakes a worker within one second; the focused queue wake/refusal tests passed ten consecutive runs.
- [x] Benchmark concurrent durable job submissions on Windows amd64 (Core i5-10400), 40 simultaneous requests per batch, 8 batches per run, 2 runs per scenario, 20 service workers, and a 3 ms fake runner. Each row represents 640 successful jobs; simulated failure rate was 0%.

  | Kind | Accounts | Cold p50 / p95 | Warm p50 / p95 |
  |---|---:|---:|---:|
  | Image | 1 | 87-90 / 139-143 ms | 116-117 / 163-165 ms |
  | Image | 5 | 70-80 / 97-106 ms | 68-71 / 111-121 ms |
  | Video | 1 | 84-85 / 141-143 ms | 101-118 / 157-162 ms |
  | Video | 5 | 88-100 / 117-127 ms | 83-92 / 151-162 ms |

  These local fake-runner numbers include concurrent HTTP acceptance, SQLite persistence, queue dispatch and scheduler contention. They exclude extension synchronization, Google Flow submission/rendering and real-account failures; keep the real upstream baseline open.
- [x] Exercise extension reconnect and cookie rotation through the real bridge listener, durable bundle write and AccountManager publication; assert runtime replacement does not overlap account jobs.
- [x] Exercise shared concurrent refresh, independent account refresh flights, cooldown failover, queue deadlines, cancellation, drain and ambiguous-submission persistence.
- [x] Verify HTTP 429 generation errors use the named bounded `account_throttle` budget; verify upload/read 429s only signal cooldown, 5xx writes become submission-unknown, and an error after durable accepted media resumes the stored media ID without a second generation call.
- [x] Add a combined durable-service integration test: after reopening SQLite, it resumes previously accepted media on account A while account A's runtime gate is held for a refresh/rotation, and proves account B completes a new job before A is released. This simulates the account-local refresh gate; extension transport/rotation itself is covered by the separate real bridge-listener test above.
- [x] Run a controlled local fake-runner benchmark and record cold/warm, image/video and one/five-account p50/p95 latency plus simulated failure rate. Production upstream latency and failure rates remain open in Phase 0.
- [ ] Run Go race tests in a supported environment with CGO and a C compiler.
- [x] Run the full Python pytest suite in an environment where pytest is installed.

### Phase 6: packaging and operational readiness

- [x] Rebuild Windows amd64, Linux amd64, macOS amd64 and macOS arm64 distribution binaries after the account wake-up, SQLite `RETURNING`, terminal model-refusal, and typed retry/outcome changes.
- [x] Add bounded p50/p95 phase latency summaries overall and grouped by job kind; retain the legacy `latency` stats field.
- [x] Add structured timing logs for service startup/listen, bridge attach, account bootstrap, job acceptance, captcha, upstream submit/media, render, download and completion.
- [x] Documented backup/restore rehearsal, service readiness checks, one-owner cutover, service-mode API configuration, monitoring signals and rollback steps in `docs/SERVICE_OPERATIONS.md`. A local pre-cutover backup and API cutover are now complete; restore rehearsal and production-shaped validation remain open.
- [x] Added a read-only deployment preflight that requires explicit service mode, checks Go health/readiness and bearer auth, validates strict-vs-cached cookie freshness, and confirms FastAPI reports a healthy service-mode engine before intake is switched.
- [x] Smoke-checked the built Windows service against temporary empty data: `/health` returned `ok`, bearer-authenticated `/internal/v1/stats` responded, and `/readyz` correctly returned 503 with zero registered accounts. This validates listener/auth/readiness behavior only, not cookie bootstrap or production readiness.
- [x] Backed up the active SQLite database plus WAL/SHM files and both account cookie bundles before cutover. SHA-256 comparisons matched; the backup DB reopened read-only with 2 accounts and 28 generation records, both cookie bundle JSON files parsed, and a SHA-256 manifest was saved. Backup: `flow-agent/data/backups/pre-service-cutover-20261014-045621`.
- [x] Rehearse service schema upgrade and backup/restore against a separate production-shaped copy. Restored the 2026-10-14 production snapshot (4 account rows, 28 generations, 0 jobs) into `flow-agent/data/restore-rehearsal-20261014-053346`; Go `stats` opened it twice, preserved the row counts and data, and the restored DB passed `integrity_check`. Existing migration tests cover adding identity columns to a pre-identity accounts table and new queue columns to an older jobs table. The extra copied cookie bundles were removed after the restore check.
- [x] Reconcile legacy account identity/history rows only when the current explicit bundle ID and exactly one active historical row share both the SAPISID fingerprint and `last_authuser`. The transaction transfers generations, request logs, uploads, attempts, pinned queued-job account IDs, cooldowns and anchors; it preserves current credits and parks/marks the old row superseded. Ambiguous candidates fail closed. Two legacy rows were reconciled on the live database, moving all 28 generation records and anchor history to the two current bundle IDs.
- [ ] Run bounded real-account image/video smoke requests after operator approval.
- [x] Stop the old FastAPI owner after confirming no established API requests or Go subprocesses; start one Go service on the configured bridge/API ports.
- [x] Switch FastAPI explicitly to service mode and run the read-only preflight plus three health/readiness/stats samples during cutover. At that time all samples remained service mode, `ready`, one ready/connected account, zero outstanding jobs and zero `submission_unknown`.
- [ ] Start and verify the workspace VS Code MCP client with the same service URL/token/output directory. The workspace launcher passed stdio smoke through Go, but the IDE host has not yet loaded/restarted the server.
- [ ] Continue monitoring queue wait, ready accounts, cookie sync age, auth failures, refresh count and `submission_unknown` under real intake.
- [ ] Exercise rollback by stopping intake, draining/reconciling accepted jobs, stopping the service, then restoring the prior backend without duplicate submissions.

## Current verification evidence

- Revalidated on 2026-10-14: `go test ./...` passed across all Go packages, `go vet ./...` passed with `GOCACHE` redirected to a writable temporary directory, and `python -m unittest tests.test_service_cutover_preflight tests.test_service_adapter -q` passed all 33 tests.
- Tightened the Python adapter after auditing the mode-selection path: startup now fails when `FLOW_ENGINE_MODE` is absent, and subprocess compatibility must be selected explicitly. The 33-test adapter/preflight suite passes with a new fail-closed mode test; README, architecture and CLI reference no longer document subprocess as the implicit default.
- Updated Claude Desktop and IDE MCP examples to include the service URL, shared token and output directory required by the fail-closed startup; all three JSON examples in `docs/MCP_GUIDE.md` parse successfully. API reference now lists `FLOW_ENGINE_MODE=service` as a requirement for native async job routes.
- Pre-cutover read-only check on 2026-10-14 found a Python listener on `127.0.0.1:8001` started 2026-10-07, before the checked-in API source timestamp of 2026-10-13; its `/health` response lacked the current `mode` field. After explicit cutover instruction, the old PID was stopped only after checking there were no established API requests or Go subprocesses. The new Go service then started on `127.0.0.1:9223` with the bridge on `9222`, and FastAPI was started on `127.0.0.1:8001` in service mode.
- Revalidated environment limits on 2026-10-14: `python -m pytest -q` cannot start because `pytest` is not installed; Go reports `CGO_ENABLED=0` with `CC=gcc`, and no `gcc`, `clang`, `zig` or `cl` executable is available, so race-enabled Go tests cannot run in this environment.
- Ran an isolated FastAPI-to-Go service integration smoke on 2026-10-14 using fresh temporary data/cookie/output directories and non-production ports. FastAPI `/health` returned `mode=service`, `engine=true`, `strict_cookie_freshness=true`, and zero ready accounts; authenticated Go stats responded; Go `/readyz` returned 503 as expected without accounts. No generation request was sent. Both staging processes were stopped, staging ports were confirmed closed, and the temporary directory was removed.
- Live local cutover on 2026-10-14: backed up the existing DB, sidecars and cookie bundles to the path recorded above; Go loaded two account identities and the extension connected, syncing one account. Three credit-bootstrap requests logged HTTP 401; after live bridge sync the service reported one ready and one connected account under strict cookie freshness. FastAPI `/health` reported service mode and engine healthy; the cutover preflight passed in the configured launch environment; three read-only samples stayed ready with zero queued/running/unknown jobs. No image/video generation request was sent, so upstream generation permissions remain unverified. The bearer token is stored outside the repository in the current Windows user's temp directory; no token value was logged.
- Runtime recheck on 2026-10-14: listeners remain active on 8001 (FastAPI), 9222 (extension bridge) and 9223 (Go API); FastAPI `/health` and Go `/readyz` both report service mode/ready with one ready and connected account and strict cookie freshness. A fresh standalone invocation of `service_cutover_preflight.py` did not pass because that shell lacked `FLOW_ENGINE_MODE`, `FLOW_SERVICE_URL` and `FLOW_INTERNAL_TOKEN`; it made no runtime changes. Re-run it from the configured launch environment for a fresh full preflight.
- Fresh preflight on 2026-10-14 passed after loading the service-mode variables and bearer token from the protected temp token file into the check process; it confirmed Go health/readiness, authenticated stats, strict cookie freshness and FastAPI service mode. The command does not change intake or submit a generation.
- MCP service-mode smoke on 2026-10-14: `.vscode/mcp.json` invokes `.vscode/run-flow-mcp.ps1`, which loads the protected temp token file and sets service mode, URL and shared output directory without putting the token in the repository. The exact workspace launcher returned valid read-only `flow_get_stats` responses through Go before and after redeploy. VS Code logs showed no launched MCP host, so IDE-side discovery/restart remains unverified.
- Account consistency recheck on 2026-10-14: before identity reconciliation, four DB rows represented two sessions under `cookie-core` and `override` identities. Paired rows shared the SAPISID fingerprint but had different cookie hashes due to rotation. After reconciliation, the two `override` rows are active with 28 total historical generations and five anchors; two `cookie-core` rows are parked/superseded with zero credits, generations, anchors or counters. Total generations remain 28. No identifiers, cookies or credit amounts were emitted.
- Corrected service balance reporting: `AccountManager.Balance` includes only registered account runtimes. Added `TestBalanceExcludesPersistedAccountsWithoutCurrentRuntimes`; package tests, full `go test ./...`, `go vet ./...`, and Windows binary build/version checks passed. The change is deployed.
- Added proof-based identity reconciliation on runtime bootstrap. It requires one unique legacy row with the same SAPISID fingerprint and `last_authuser`, migrates all related account references transactionally, retains current credits, and leaves the legacy row parked/superseded. Tests cover ownership transfer, media linkage, cooldown merge, pinned job rewrites, idempotency, ambiguous candidates, missing fingerprints and differing authuser indexes.
- Controlled redeploy on 2026-10-14: before shutdown the Go/FastAPI checks passed, outstanding and `submission_unknown` were zero, and a fresh SQLite online backup plus both cookie bundles passed integrity verification (4 account rows, 28 generations) at `flow-agent/data/backups/pre-identity-reconcile-20261014-052604`. FastAPI and Go exited after Ctrl+Break while their consoles contained only each target and the signaling shell. The prior binary is in that backup; deployed binary hash matches the tested build. Restarted Go on 9223/bridge 9222 and FastAPI on 8001 in service mode. Startup reconciled two legacy rows with zero skips, moving all 28 generations and five anchors to the two active bundle IDs. Legacy rows are parked/superseded without credits or counters. Extension reconnected; health, readiness, preflight, balance and read-only MCP smoke passed. Balance returns two runtime rows and zero unread credits. Strict freshness still leaves one account ready/connected.
- Authuser-safety redeploy on 2026-10-14: reconciliation now also requires matching `last_authuser`, with a regression test proving equal session fingerprints at different authuser indexes do not merge. Focused reconciliation tests, full `go test ./...`, `go vet ./...` and Windows build passed. Before redeploy the authenticated queue showed zero outstanding and `submission_unknown`; a fresh SQLite online backup passed integrity check (`4` account rows, `28` generations, `0` jobs) at `flow-agent/data/backups/pre-authuser-reconcile-20261014-053127`. The new binary hash is `1F5E0D321D5AEF81F915EFC47F9739F7D41203192F44D84D9ABC184A7DE01C45`. Go restarted on 9223/bridge 9222 with one connected and ready account; FastAPI restarted on 8001 in service mode. Fresh cutover preflight passed and queue remains empty. No generation request was sent.
- Restore/schema rehearsal on 2026-10-14: restored the integrity-checked snapshot into the separate staging path above, opened it twice with the deployed Go CLI (thereby rerunning idempotent schema migration), and confirmed `28` generations, `4` account rows, `0` jobs and `integrity_check=ok`. The focused migration tests in `internal/store/db_test.go` also cover an older accounts schema missing identity columns and an older jobs schema missing queue columns. Staging data remains isolated from the live service; duplicate cookie bundle copies were removed after verification.
- Latest idle monitoring sample on 2026-10-14: three samples five seconds apart all showed Go `ok`, FastAPI `mode=service`, one ready/connected account, zero outstanding and zero `submission_unknown`. Two cookie bundle ages were present; the oldest advanced from about 552 to 562 seconds during the idle interval. This is a short idle check, not sustained monitoring under real intake.
- Workspace MCP stdio smoke on 2026-10-14 after redeploy: launched the exact `.vscode/run-flow-mcp.ps1` wrapper, completed MCP initialize, listed all five tools and called read-only `flow_get_stats` through Go successfully. VS Code processes are running, but the `code --status` command failed with `spawn EPERM`; current IDE-side MCP discovery/restart remains unverified.
- Earlier full Python pytest attempts could not start because `pytest` was absent from the project venv. This was resolved by installing the declared test tools into a temporary `%TEMP%` directory without modifying the project environment.
- Re-ran `python -m unittest tests.test_service_cutover_preflight tests.test_service_adapter -q` from `flow-agent/` on 2026-10-14: all 33 tests passed.
- Installed `pytest` and `pytest-asyncio` into `%TEMP%` and ran the complete suite from `flow-agent/` on 2026-10-14: 114 passed. The run exposed two POSIX-specific executable-path expectations, now replaced with a platform-neutral test executable name, and an MCP upload fake that recorded an absent optional account pin as `None`; the fake now records only supplied options. Added coverage that an explicitly pinned MCP upload preserves its account ID. One existing Starlette/httpx deprecation warning remains.
- Cookie freshness flow updated on 2026-10-15: service jobs no longer call `syncLatest` or pull cookies from the extension before execution. Each service engine reads its configured account bundle from disk through `Engine.Jar`; extension rotation events still trigger a scoped bridge sync, durable bundle write, and account-runtime reload. Strict freshness requires the owning extension to stay connected, while `FLOW_COOKIE_FRESHNESS=cached` permits an offline persisted bundle. A 401 session-recovery path can still refresh the browser session, persist its bundle, and reload the cookie jar from that file.
- Rebuilt all four Go distribution targets from the authuser-safe source on 2026-10-14: Windows amd64, Linux amd64, macOS amd64 and macOS arm64. The Windows build was produced in `%TEMP%` so the running executable was not replaced; its SHA-256 matches the deployed `flow-windows.exe` (`1F5E0D321D5AEF81F915EFC47F9739F7D41203192F44D84D9ABC184A7DE01C45`). `go version -m` confirms the target OS/architecture and `vcs.modified=true` for the builds.
- Re-ran `go test ./...` and `go vet ./...` from the Go module on 2026-10-14 after the authuser reconciliation change; both passed.
- Ran the extension bridge contract suite on 2026-10-14 with `node test/dispatch.test.mjs`: all 35 checks passed, including cookie-rotation burst coalescing, Flow cookie scope, extension reconnect and bundle compatibility.
- Race-detector environment recheck on 2026-10-14: no native C compiler is installed. A temporary Zig compiler was able to compile/link the Windows race binaries after adding the missing synchronization API import library, but the Windows ThreadSanitizer runtime then failed before tests with `failed to allocate ... bytes` (error 87). An installed Ubuntu WSL distribution cannot start because hardware virtualization is disabled. The race suite therefore remains unverified and requires a supported CI/Linux host.
- Re-ran the exact workspace MCP launcher on 2026-10-14 after the MCP upload option change: initialize succeeded, all five tools were listed and read-only `flow_get_stats` returned through the live Go service.
- Isolated rollback-mode smoke on 2026-10-14: started a Go service and FastAPI against fresh temporary directories and random loopback ports, verified FastAPI health in `service` mode, stopped that staging API, restarted it in explicit `subprocess` mode and verified health again. The staging Go service had zero accounts; no generation or queue job was submitted, and all staging processes and files were removed. This verifies backend mode restoration only; the live queue-drain/accepted-job rollback rehearsal remains open.
- Updated `docs/SERVICE_OPERATIONS.md` to state that strict freshness requires a live extension connection from each account's browser profile; a persisted bundle with its profile offline is not schedulable.
- `go test ./...` passed after latency-by-kind stats and operational timing logs were added.
- The service package tests passed after adding immediate queue-worker wake signaling and terminal policy-refusal coverage.
- The latest full `go test ./...` passed with the queue wake-up implementation; `go vet ./...` passed before rebuilding all four distribution binaries.
- The latest full Go suite and `go vet ./...` passed after pool waiter signaling and single-statement SQLite job acceptance; the 26-test Python adapter suite passed again.
- `TestRestartedAcceptedMediaWaitsForAccountRefreshWhileOtherAccountServes` passed five consecutive runs; it reopens the durable database, holds account A's runtime gate, confirms accepted-media recovery blocks only A, then releases A and confirms both jobs reach success.
- `go vet ./...` passed.
- After adding the combined accepted-media/account-refresh isolation test, the full `go test ./...` suite and `go vet ./...` passed again.
- After making model-access refusals terminal before internal captcha retries, `go test ./...` and `go vet ./...` passed; the focused refusal/retry tests passed five consecutive runs.
- HTTP status classification passed focused batchexecute tests five consecutive runs; durable HTTP-throttle and accepted-media recovery tests passed ten consecutive runs; resumable video upload ambiguity/cooldown tests passed repeated runs.
- The full Go suite and `go vet ./...` passed after adding typed HTTP throttle/unknown outcomes, ambiguous upload-video handling, and accepted-media resume-on-error behavior.
- The 26-test Python service-adapter suite passed again after the retry/outcome changes; all four distribution binaries were rebuilt, and the Windows binary's `version` command succeeded.
- The rebuilt Windows service passed a local no-account smoke check on temporary paths: health and authenticated stats worked, while readiness stayed unavailable until an account was attached.
- `python -m unittest tests.test_service_cutover_preflight tests.test_service_adapter -q` passed: 33 standard-library tests, including ready/fail-closed/cached-cookie/API-mode and stats-auth preflight cases.
- Installing the optional Python test dependencies in the project venv was attempted, but the configured pip index returned no available `pytest` distribution; the full pytest suite remains unrun.
- All four distribution binaries were rebuilt after the refusal change. The Windows amd64 binary's `version` command succeeded and `go version -m` reported `vcs.modified=true`.
- The 26-test Python service-adapter unittest suite passed. Full unittest discovery could not import four pytest-based modules because `pytest` is absent from the project venv; `python -m pytest` is likewise unavailable.
- `go test -race ./internal/service ./internal/batchexecute` was attempted with `CGO_ENABLED=1`; it could not build because `gcc` is not installed or available on `PATH`.
- `python -m unittest tests.test_service_adapter -q` passed: 26 tests. This is the standard-library adapter contract suite, not the full pytest suite.
- The stats endpoint test checks both the backward-compatible `latency` field and new `latency_by_kind` field.
- `go version -m` confirmed the rebuilt Windows binary is a modified working-tree build; all four distribution targets were rebuilt.
- `git diff --check` reported no whitespace errors; Git emitted only configured LF/CRLF conversion warnings.

## Rollout gate

Implementation, local fake-upstream verification and this local service-mode cutover do not establish production readiness. The operator approved one image and one video canary on 2026-10-14; both were submitted once through the FastAPI service and failed, so successful production generation remains unverified. Restore/schema rehearsal is complete, while IDE MCP client startup, sustained monitoring and rollback remain open. The strict-fresh runtime reports two registered accounts but only one live extension connection and one ready account.

### Real-account canary record (2026-10-14)

- Preflight: FastAPI health was `ok` in `service` mode; Go engine was healthy; strict cookie freshness was enabled; one account was ready and connected; durable queue had `outstanding=0` and `submission_unknown=0`.
- Image: submitted exactly once as async job `f17318ca-bcdc-4bc7-953e-69e0eb6642fe` with an idempotency key, count 1, model `narwhal`, aspect `1:1`. It selected account `7735dc3ed01c`, reached the image RPC, then failed because the RPC returned no image frames. The engine error suggests the model may not be available to that account or the project may not belong to that account. No retry was made.
- Video: submitted exactly once as async job `7c85ceb1-ddaf-46dd-90eb-861d120bf5fc` with an idempotency key, count 1, duration `4s`, quality `360p`. It failed during account preparation because the scheduler selected account `c2193113e3fd`, whose extension was offline and required live cookie sync. No video RPC was reached and no retry was made.
- Postflight: queue drained to `outstanding=0`, `submission_unknown=0`, `failed=2`; health continued to report one ready/connected account. The mismatch between that readiness and the video job's selected offline account is a rollout blocker to investigate before more generation requests.
