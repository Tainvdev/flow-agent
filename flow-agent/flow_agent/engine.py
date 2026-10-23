"""Async adapter for the persistent Go service and legacy Flow CLI.

In service mode, requests become durable jobs handled by the shared account
scheduler. In subprocess mode, each generation runs one `./bin/flow` process,
which writes its result to
stdout as JSON and its diagnostics to stderr, so this runs the process, keeps the
two apart, and hands back the parsed object.

Nothing here reimplements the engine. The project resolution, the reCAPTCHA
token, the rate-limit cooldown, the account rotation and the SQLite writes all
already exist in the binary — this is a caller, not a second engine.
"""

from __future__ import annotations

import asyncio
import json
import os
import sqlite3
import uuid
from pathlib import Path
from typing import Any, Optional

#: How long a generation may take before it is killed. A video render polls for
#: minutes, so this is generous by design rather than a hang guard.
DEFAULT_TIMEOUT = 900
SERVICE_PAYLOAD_VERSION = 1


class EngineError(RuntimeError):
    """A Flow engine invocation that did not succeed.

    Carries the process's own words rather than a paraphrase. The engine's
    failures are specific — a spent token, an empty wallet, a session that needs
    refreshing — and each has a different fix, so the message is passed through
    and both streams are kept for a caller that wants to look.
    """

    def __init__(
        self, command: list[str], returncode: int, stdout: str, stderr: str,
        job_id: Optional[str] = None, idempotency_key: Optional[str] = None,
    ):
        self.command = command
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr
        self.job_id = job_id
        self.idempotency_key = idempotency_key

        detail = (stderr or "").strip() or (stdout or "").strip()
        super().__init__(
            f"{' '.join(command)} exited {returncode}: {detail[-800:]}"
            if detail
            else f"{' '.join(command)} exited {returncode}"
        )


def extract_json(stdout: str) -> dict | list:
    """Pull the result value out of a command's stdout.

    Not `json.loads(stdout)`: the commands print prose before the result.
    `generate` announces what it is about to do, `upload-video` announces what it
    is resolving, and both then marshal the result with `json.MarshalIndent` —
    which puts the opening brace alone on a line. So the last such line is where
    the value starts.

    **Both shapes occur.** `generate` and `upload-video` print an object;
    `projects --json` prints a bare array. Anchoring on either delimiter covers
    the two.

    Two properties make the anchor trustworthy, and both are load-bearing:

    * **The anchor is a line that is *only* the delimiter** (or a line that is a
      complete value on its own). A progress line such as `[1/3] rendering`
      starts with `[` and would otherwise be read as the start of an array.
    * **The value must run to the end of stdout.** The JSON is always the last
      thing a command prints, so requiring the parse to consume everything
      rejects an inner object. Without this, `projects --json` would return its
      *first project* rather than the array of them — the scan runs backwards
      and hits `{` inside the array before it reaches `[`. That failure is
      silent and shape-valid, which is why the rule is asserted rather than
      assumed.

    Raises ValueError when no value parses, which the caller turns into an
    EngineError carrying the command that produced it.
    """
    lines = stdout.splitlines()
    for index in range(len(lines) - 1, -1, -1):
        stripped = lines[index].strip()
        if not stripped.startswith(("{", "[")):
            continue

        # An indented marshal puts the delimiter alone on its line, so the value
        # is everything from here down. A compact value (`[]`, `{}`, a one-line
        # object) is its own line and nothing more.
        candidate = "\n".join(lines[index:]) if stripped in ("{", "[") else stripped

        try:
            parsed = json.loads(candidate)
        except json.JSONDecodeError:
            continue
        if isinstance(parsed, (dict, list)):
            return parsed

    raise ValueError("no JSON result found on stdout")


class FlowEngine:
    """Runs the flow-go binary and parses what it prints.

    Every path is resolved relative to this package's directory, which is also
    the working directory each invocation runs in — the engine finds `cookies/`,
    `data/` and `output/` relative to its own cwd, so pointing it anywhere else
    would silently use a different set of accounts.
    """

    @staticmethod
    def _find_binary(root: Path, explicit: Optional[str | Path] = None) -> Path:
        """Locate the platform-appropriate engine executable."""
        if explicit:
            return Path(explicit)

        import platform
        system = platform.system().lower()
        machine = platform.machine().lower()
        bin_dir = root / "bin"

        # Platform priority candidates
        if "windows" in system:
            candidates = ["flow-windows.exe", "flow.exe", "flow"]
        elif "darwin" in system:
            if machine in ("arm64", "aarch64"):
                candidates = ["flow-macos-arm64", "flow-macos", "flow", "flow-darwin"]
            else:
                candidates = ["flow-macos", "flow", "flow-darwin"]
        else:  # Linux / Unix
            candidates = ["flow-linux", "flow", "flow-x86_64"]

        for name in candidates:
            candidate_path = bin_dir / name
            if candidate_path.exists():
                return candidate_path

        default_name = "flow-windows.exe" if "windows" in system else "flow"
        return bin_dir / default_name

    def __init__(
        self,
        root: Optional[str | Path] = None,
        binary: Optional[str | Path] = None,
        db_path: Optional[str | Path] = None,
        output_dir: Optional[str | Path] = None,
        timeout: int = DEFAULT_TIMEOUT,
        service_url: Optional[str] = None,
        service_token: Optional[str] = None,
        mode: Optional[str] = None,
    ) -> None:
        if root:
            self.root = Path(root).resolve()
        else:
            pkg_dir = Path(__file__).resolve().parent
            if (pkg_dir / "bin").exists():
                self.root = pkg_dir
            elif (pkg_dir.parent / "bin").exists():
                self.root = pkg_dir.parent
            else:
                self.root = pkg_dir.parent

        self.binary = self._find_binary(self.root, binary)
        # Keep Python/FastAPI on the same SQLite file as the Go service.  The
        # service has always supported FLOW_DB_PATH and FLOW_DATA_DIR; without
        # reading them here, the dashboard and /stats silently looked under
        # the Python package directory instead.
        configured_db = db_path or os.getenv("FLOW_DB_PATH")
        if configured_db:
            self.db_path = Path(configured_db).resolve()
        else:
            configured_data = os.getenv("FLOW_DATA_DIR")
            data_dir = Path(configured_data) if configured_data else self.root / "data"
            self.db_path = (data_dir / "flow.db").resolve()
        configured_output = output_dir or os.getenv("FLOW_OUTPUT_DIR")
        self.output_dir = Path(configured_output).resolve() if configured_output else self.root / "output"
        self.timeout = timeout
        configured_url = service_url if service_url is not None else os.getenv("FLOW_SERVICE_URL", "")
        self.service_url = configured_url.strip().rstrip("/")
        self.service_token = service_token or os.getenv("FLOW_INTERNAL_TOKEN", "")
        raw_mode = mode if mode is not None else os.getenv("FLOW_ENGINE_MODE")
        configured_mode = (raw_mode or "").strip().lower()
        if configured_mode and configured_mode not in {"service", "subprocess"}:
            raise ValueError("FLOW_ENGINE_MODE must be 'service' or 'subprocess'")
        if configured_mode:
            self.engine_mode = configured_mode
        elif service_url is not None:
            # Supplying a service URL in code is an explicit service choice.
            self.engine_mode = "service" if self.service_url else "subprocess"
        elif self.service_url:
            raise ValueError("set FLOW_ENGINE_MODE=service when FLOW_SERVICE_URL is configured")
        else:
            raise ValueError(
                "FLOW_ENGINE_MODE must be explicitly set to 'service' or 'subprocess'"
            )
        if self.engine_mode == "service" and not self.service_url:
            raise ValueError("FLOW_SERVICE_URL is required when FLOW_ENGINE_MODE=service")
        if self.engine_mode == "service" and not self.service_token:
            raise ValueError("FLOW_INTERNAL_TOKEN is required when FLOW_ENGINE_MODE=service")
        if self.engine_mode == "subprocess" and self.service_url:
            raise ValueError("FLOW_SERVICE_URL is set while FLOW_ENGINE_MODE=subprocess; remove one setting")
        self._service_http: Any = None

    async def _service_client(self):
        """Return the process-local pooled client for the persistent Go service."""
        if self._service_http is None:
            import httpx

            self._service_http = httpx.AsyncClient(
                timeout=httpx.Timeout(15.0, connect=5.0),
                limits=httpx.Limits(max_connections=100, max_keepalive_connections=20),
            )
        return self._service_http

    async def aclose(self) -> None:
        client, self._service_http = self._service_http, None
        if client is not None:
            await client.aclose()

    async def _recover_service_submission(
        self, client: Any, key: str, command: list[str], wait_seconds: float = 2.0
    ) -> dict:
        """Resolve a lost POST acknowledgement by its durable idempotency key."""
        import httpx

        deadline = asyncio.get_running_loop().time() + max(0.0, wait_seconds)
        last_error = ""
        while asyncio.get_running_loop().time() < deadline:
            try:
                response = await client.get(
                    f"{self.service_url}/internal/v1/jobs/by-key",
                    params={"key": key},
                    headers={"Authorization": f"Bearer {self.service_token}"},
                )
            except httpx.HTTPError as exc:
                last_error = str(exc)
            else:
                if response.status_code == 200:
                    try:
                        accepted = response.json()
                    except ValueError as exc:
                        last_error = f"invalid job lookup response: {exc}"
                    else:
                        if isinstance(accepted, dict) and accepted.get("job_id"):
                            return accepted
                        last_error = "job lookup response omitted job_id"
                elif response.status_code != 404 and response.status_code < 500:
                    last_error = f"job lookup returned {response.status_code}: {response.text}"
                else:
                    last_error = f"job lookup returned {response.status_code}"
            await asyncio.sleep(min(0.1, max(0.01, deadline - asyncio.get_running_loop().time())))

        detail = f"service submit outcome is unknown; retry with the same Idempotency-Key {key}"
        if last_error:
            detail += f" (lookup: {last_error})"
        raise EngineError(command, -1, "", detail, idempotency_key=key)

    # -- process plumbing -------------------------------------------------

    def _command(self, *args: Any) -> list[str]:
        """The argv for one invocation, dropping the arguments that were not given.

        Callers pass `None` or `""` for the options they are not using, and this
        is the one place that turns "not given" into "not on the command line" —
        so no call site has to build its argv conditionally.
        """
        return [str(self.binary)] + [str(a) for a in args if a not in (None, "")]

    async def _run(self, *args: Any, timeout: Optional[int] = None) -> tuple[str, str]:
        """Run one command and return (stdout, stderr), raising on a non-zero exit."""
        if not self.binary.exists():
            raise EngineError(
                self._command(*args), 127, "", f"no engine binary at {self.binary}"
            )

        import os
        if hasattr(os, "chmod") and not os.access(self.binary, os.X_OK):
            try:
                os.chmod(self.binary, 0o755)
            except OSError:
                pass

        command = self._command(*args)
        limit = timeout or self.timeout

        proc = await asyncio.create_subprocess_exec(
            *command,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
            cwd=str(self.root),
        )
        try:
            stdout, stderr = await asyncio.wait_for(proc.communicate(), limit)
        except asyncio.TimeoutError:
            proc.kill()
            await proc.wait()
            raise EngineError(
                command, -1, "", f"did not finish within {limit}s and was killed"
            ) from None

        out = stdout.decode("utf-8", "replace")
        err = stderr.decode("utf-8", "replace")
        if proc.returncode != 0:
            raise EngineError(command, proc.returncode or 1, out, err)
        return out, err

    async def _run_json(self, *args: Any, timeout: Optional[int] = None) -> dict:
        """Run one command and return its parsed result object."""
        out, _ = await self._run(*args, timeout=timeout)
        try:
            return extract_json(out)
        except ValueError as exc:
            raise EngineError(self._command(*args), 0, out, str(exc)) from exc

    async def _run_service_job(
        self, kind: str, payload: dict[str, Any], timeout: Optional[int] = None,
        idempotency_key: Optional[str] = None,
    ) -> dict:
        """Submit once to the persistent Go service and poll its durable job."""
        if not self.service_token:
            raise EngineError([], 503, "", "FLOW_INTERNAL_TOKEN is required in service mode")

        payload = self._normalize_service_account(payload)

        import httpx

        command = [self.service_url, kind]
        deadline = asyncio.get_running_loop().time() + (timeout or self.timeout)
        headers = {
            "Authorization": f"Bearer {self.service_token}",
            "Idempotency-Key": idempotency_key or str(uuid.uuid4()),
        }
        client = await self._service_client()
        try:
            response = await client.post(
                f"{self.service_url}/internal/v1/jobs",
                headers=headers,
                json={"kind": kind, "payload": payload, "payload_version": SERVICE_PAYLOAD_VERSION},
            )
        except httpx.HTTPError as exc:
            # Recover an acknowledgement lost after the durable commit, without
            # ever submitting the generation a second time.
            try:
                accepted = await self._recover_service_submission(
                    client, headers["Idempotency-Key"], command,
                    min(2.0, max(0.0, deadline - asyncio.get_running_loop().time())),
                )
            except EngineError as recovery_error:
                raise recovery_error from exc
        else:
            if response.status_code >= 500:
                try:
                    accepted = await self._recover_service_submission(
                        client, headers["Idempotency-Key"], command,
                        min(2.0, max(0.0, deadline - asyncio.get_running_loop().time())),
                    )
                except EngineError as recovery_error:
                    raise EngineError(
                        command, response.status_code, response.text,
                        str(recovery_error), idempotency_key=headers["Idempotency-Key"],
                    ) from recovery_error
            elif response.status_code not in (200, 202):
                raise EngineError(command, response.status_code, "", response.text)
            else:
                try:
                    accepted = response.json()
                except ValueError as exc:
                    raise EngineError(
                        command, response.status_code, response.text,
                        "invalid job acknowledgement", idempotency_key=headers["Idempotency-Key"],
                    ) from exc
        try:
            job_id = accepted["job_id"]
        except (KeyError, TypeError) as exc:
            raise EngineError(
                command, 502, "", "invalid recovered job acknowledgement",
                idempotency_key=headers["Idempotency-Key"],
            ) from exc

        while True:
            if asyncio.get_running_loop().time() >= deadline:
                raise EngineError(
                    command, -1, "", f"job {job_id} is still running; query its status URL",
                    job_id=job_id,
                )
            try:
                status_response = await client.get(
                    f"{self.service_url}/internal/v1/jobs/{job_id}", headers=headers
                )
            except httpx.HTTPError as exc:
                remaining = deadline - asyncio.get_running_loop().time()
                if remaining <= 0:
                    raise EngineError(
                        command, -1, "", f"could not read job {job_id} before the wait deadline: {exc}",
                        job_id=job_id,
                    ) from exc
                await asyncio.sleep(min(0.5, max(0.05, remaining)))
                continue
            if status_response.status_code >= 500:
                remaining = deadline - asyncio.get_running_loop().time()
                if remaining <= 0:
                    raise EngineError(
                        command, status_response.status_code, "", status_response.text,
                        job_id=job_id,
                    )
                await asyncio.sleep(min(0.5, max(0.05, remaining)))
                continue
            if status_response.status_code != 200:
                raise EngineError(
                    command, status_response.status_code, "", status_response.text,
                    job_id=job_id,
                )
            try:
                status = status_response.json()
            except ValueError as exc:
                raise EngineError(command, status_response.status_code, status_response.text, "invalid job status") from exc

            state = status.get("status")
            if state == "succeeded":
                result = status.get("result")
                if isinstance(result, dict):
                    return result
                return {"result": result, "job_id": job_id}
            if state == "partial":
                return status.get("result") or status
            if state in ("failed", "submission_unknown", "cancelled"):
                detail = status.get("error") or f"job ended with status {state}"
                error_code = status.get("error_code", 502)
                if isinstance(error_code, bool) or not isinstance(error_code, int):
                    error_code = 502
                if not 400 <= error_code <= 599:
                    error_code = 502
                raise EngineError(command, error_code, "", detail, job_id=job_id)
            await asyncio.sleep(min(0.5, max(0.05, deadline - asyncio.get_running_loop().time())))

    async def _service_get(self, path: str, params: Optional[dict[str, Any]] = None) -> Any:
        if not self.service_token:
            raise EngineError([], 503, "", "FLOW_INTERNAL_TOKEN is required in service mode")
        import httpx

        command = [self.service_url, path]
        try:
            client = await self._service_client()
            response = await client.get(
                f"{self.service_url}{path}",
                params=params,
                headers={"Authorization": f"Bearer {self.service_token}"},
            )
        except httpx.HTTPError as exc:
            raise EngineError(command, -1, "", str(exc)) from exc
        if response.status_code != 200:
            raise EngineError(command, response.status_code, "", response.text)
        try:
            return response.json()
        except ValueError as exc:
            raise EngineError(command, response.status_code, response.text, "invalid service response") from exc

    async def _service_patch(self, path: str, payload: dict[str, Any]) -> Any:
        if self.engine_mode != "service" or not self.service_token:
            raise EngineError([], 503, "", "runtime settings require configured service mode")
        import httpx

        command = [self.service_url, "PATCH", path]
        try:
            response = await (await self._service_client()).patch(
                f"{self.service_url}{path}",
                headers={"Authorization": f"Bearer {self.service_token}"},
                json=payload,
            )
        except httpx.HTTPError as exc:
            raise EngineError(command, -1, "", str(exc)) from exc
        if response.status_code != 200:
            raise EngineError(command, response.status_code, "", response.text)
        try:
            return response.json()
        except ValueError as exc:
            raise EngineError(command, response.status_code, response.text, "invalid service response") from exc

    async def enqueue_service_job(
        self, kind: str, payload: dict[str, Any], idempotency_key: Optional[str] = None
    ) -> dict:
        """Persist a job and return its identifier without waiting for generation."""
        if self.engine_mode != "service" or not self.service_token:
            raise EngineError([], 503, "", "native asynchronous jobs require configured service mode")
        payload = self._normalize_service_account(payload)
        import httpx

        key = idempotency_key or str(uuid.uuid4())
        command = [self.service_url, "submit", kind]
        try:
            response = await (await self._service_client()).post(
                f"{self.service_url}/internal/v1/jobs",
                headers={"Authorization": f"Bearer {self.service_token}", "Idempotency-Key": key},
                json={"kind": kind, "payload": payload, "payload_version": SERVICE_PAYLOAD_VERSION},
            )
        except httpx.HTTPError as exc:
            try:
                return await self._recover_service_submission(
                    await self._service_client(), key, command
                )
            except EngineError as recovery_error:
                raise recovery_error from exc
        if response.status_code >= 500:
            try:
                return await self._recover_service_submission(
                    await self._service_client(), key, command
                )
            except EngineError as recovery_error:
                raise EngineError(
                    command, response.status_code, response.text, str(recovery_error),
                    idempotency_key=key,
                ) from recovery_error
        if response.status_code not in (200, 202):
            raise EngineError(command, response.status_code, "", response.text)
        try:
            return response.json()
        except ValueError as exc:
            raise EngineError(
                command, response.status_code, response.text, "invalid job acknowledgement",
                idempotency_key=key,
            ) from exc

    async def get_service_job(self, job_id: str) -> dict:
        job = await self._service_get(f"/internal/v1/jobs/{job_id}")
        job["events_url"] = f"/api/v1/jobs/{job_id}/events"
        return job

    async def open_service_job_events(self, job_id: str, last_event_id: Optional[str] = None):
        """Open the authenticated Go SSE stream for a public job-events route."""
        if self.engine_mode != "service" or not self.service_token:
            raise EngineError([], 503, "", "job events require configured service mode")
        import httpx

        command = [self.service_url, "events", job_id]
        headers = {
            "Authorization": f"Bearer {self.service_token}",
            "Accept": "text/event-stream",
        }
        if last_event_id is not None:
            headers["Last-Event-ID"] = last_event_id
        client = await self._service_client()
        request = client.build_request(
            "GET", f"{self.service_url}/internal/v1/jobs/{job_id}/events", headers=headers
        )
        try:
            response = await client.send(request, stream=True)
        except httpx.HTTPError as exc:
            raise EngineError(command, -1, "", str(exc), job_id=job_id) from exc
        if response.status_code != httpx.codes.OK:
            body = (await response.aread()).decode("utf-8", errors="replace")
            await response.aclose()
            raise EngineError(command, response.status_code, "", body, job_id=job_id)
        if "text/event-stream" not in response.headers.get("content-type", "").lower():
            body = (await response.aread()).decode("utf-8", errors="replace")
            await response.aclose()
            raise EngineError(command, 502, "", "service returned a non-SSE event response: " + body, job_id=job_id)
        return response

    async def cancel_service_job(self, job_id: str) -> None:
        if self.engine_mode != "service" or not self.service_token:
            raise EngineError([], 503, "", "job cancellation requires configured service mode")
        import httpx

        command = [self.service_url, "cancel", job_id]
        try:
            response = await (await self._service_client()).post(
                f"{self.service_url}/internal/v1/jobs/{job_id}/cancel",
                headers={"Authorization": f"Bearer {self.service_token}"},
            )
        except httpx.HTTPError as exc:
            raise EngineError(command, -1, "", str(exc)) from exc
        if response.status_code not in (httpx.codes.NO_CONTENT, httpx.codes.ACCEPTED):
            raise EngineError(command, response.status_code, "", response.text)

    # -- generation -------------------------------------------------------

    async def generate_image(
        self,
        prompt: str,
        aspect: str = "1:1",
        count: int = 1,
        model: str = "narwhal",
        all_accounts: bool = False,
        cookies: Optional[str] = None,
        download: bool = True,
        timeout: Optional[int] = None,
        idempotency_key: Optional[str] = None,
        account_id: Optional[str] = None,
    ) -> dict:
        """Generate an image and return the engine's result object.

        `aspect` and `count` are the CLI's trailing shorthand, so they go on the
        command line as tokens rather than as flags — `1:1`, `x2` — which is the
        form the engine documents and accepts.

        The result carries `job_id`, `account_id`, `project_id`, `model`,
        `media`, `files` and `status`. `files` is absent when nothing was written,
        which happens with `download=False` and on a refusal alike, so a caller
        that needs the bytes should read it and not assume it.
        """
        if self.engine_mode == "service":
            payload: dict[str, Any] = {
                "prompt": prompt, "aspect": aspect, "count": count, "model": model,
                "all_accounts": all_accounts,
            }
            if account_id:
                payload["account_id"] = account_id
            if cookies:
                payload["cookies"] = cookies
            if not download:
                payload["no_download"] = True
            return await self._run_service_job("image", payload, timeout, idempotency_key)

        cookies = self._subprocess_cookie_selector(cookies, account_id)
        args: list[Any] = ["image", prompt, aspect]
        if count and count > 1:
            args.append(f"x{count}")
        if model:
            args += ["--model", model]
        if all_accounts:
            # The CLI keeps its human-readable table by default. Python callers
            # need the machine-readable aggregate so FastAPI/MCP can preserve
            # results, files and partial failures across accounts.
            args += ["--all", "--json"]
        if cookies:
            args += ["--cookies", cookies]
        if not download:
            args.append("--no-download")

        result = await self._run_json(*args, timeout=timeout)
        if all_accounts and result.get("status") == "failed":
            raise EngineError(self._command(*args), 502, json.dumps(result), "all account generation failed")
        return result

    async def generate_video(
        self,
        prompt: str,
        aspect: str = "landscape",
        duration: str = "8s",
        quality: str = "720p",
        count: int = 1,
        start_image: Optional[str] = None,
        all_accounts: bool = False,
        cookies: Optional[str] = None,
        download: bool = True,
        timeout: Optional[int] = None,
        idempotency_key: Optional[str] = None,
        account_id: Optional[str] = None,
    ) -> dict:
        """Generate a video and return the engine's result object.

        The result carries `media_ids`, `urls`, `files`, `credits_remaining`,
        `quality` and `status`. A render is asynchronous, so with `download=False`
        the ids are returned before the file exists.
        """
        if self.engine_mode == "service":
            payload: dict[str, Any] = {
                "prompt": prompt, "aspect": aspect, "duration": duration,
                "quality": quality, "count": count, "all_accounts": all_accounts,
            }
            if account_id:
                payload["account_id"] = account_id
            if start_image:
                payload["start_image"] = start_image
            if cookies:
                payload["cookies"] = cookies
            if not download:
                payload["no_download"] = True
            return await self._run_service_job("video", payload, timeout, idempotency_key)

        cookies = self._subprocess_cookie_selector(cookies, account_id)
        args: list[Any] = ["generate", prompt, aspect, duration, quality]
        if count and count > 1:
            args.append(f"x{count}")
        if start_image:
            args += ["--start-image", start_image]
        if all_accounts:
            args += ["--all", "--json"]
        if cookies:
            args += ["--cookies", cookies]
        if not download:
            args.append("--no-download")

        result = await self._run_json(*args, timeout=timeout)
        if all_accounts and result.get("status") == "failed":
            raise EngineError(self._command(*args), 502, json.dumps(result), "all account generation failed")
        return result

    async def upload_video(
        self,
        file_path: str,
        cookies: Optional[str] = None,
        force: bool = False,
        timeout: Optional[int] = None,
        idempotency_key: Optional[str] = None,
        account_id: Optional[str] = None,
    ) -> dict:
        """Upload a local video and return the engine's result object.

        The id is `result["media_id"]`, and `result["cached"]` says whether the
        file was already in the project — the engine keys that on a SHA-256 of
        the bytes, so re-uploading the same file is free.
        """
        if self.engine_mode == "service":
            payload: dict[str, Any] = {"file_path": file_path, "force": force}
            if account_id:
                payload["account_id"] = account_id
            if cookies:
                payload["cookies"] = cookies
            return await self._run_service_job("upload-video", payload, timeout, idempotency_key)

        cookies = self._subprocess_cookie_selector(cookies, account_id)
        args: list[Any] = ["upload-video", file_path]
        if force:
            args.append("--force-upload")
        if cookies:
            args += ["--cookies", cookies]

        return await self._run_json(*args, timeout=timeout)

    async def service_available(self) -> bool:
        """Whether the configured Go service answers its local health check."""
        return (await self.service_health()).get("status") == "ok"

    async def service_health(self) -> dict[str, Any]:
        """Read service readiness details without exposing credentials."""
        if self.engine_mode != "service":
            return {"status": "ok" if self.binary.exists() else "unavailable", "mode": "subprocess"}
        import httpx

        try:
            response = await asyncio.wait_for(
                (await self._service_client()).get(f"{self.service_url}/health"), timeout=3.0
            )
            if response.status_code == 200:
                value = response.json()
                if isinstance(value, dict):
                    return {"status": "ok", "mode": "service", **value}
        except (httpx.HTTPError, asyncio.TimeoutError, ValueError):
            pass
        return {"status": "unavailable", "mode": "service"}

    # -- reads ------------------------------------------------------------

    async def get_balance(self, refresh: bool = False) -> dict:
        """Report every account's credits.

        `refresh` probes upstream first. Either way the figures are read from the
        database, because `balance --refresh` prints a table rather than JSON —
        the probe's job is to write the database, and that is the interface.
        """
        if self.engine_mode == "service":
            return await self._service_get("/internal/v1/balance", {"refresh": str(refresh).lower()})
        if refresh:
            await self._run("balance", "--refresh")
        return await asyncio.to_thread(self._read_balance)

    async def get_stats(self) -> dict:
        """Report generation history and engine state, read from the database."""
        if self.engine_mode == "service":
            return await self._service_get("/internal/v1/stats")
        return await asyncio.to_thread(self._read_stats)

    async def get_monitor(self, limit: int = 50, status: str = "", kind: str = "",
                          page: Optional[int] = None, page_size: Optional[int] = None,
                          request_page: Optional[int] = None, request_page_size: Optional[int] = None) -> dict:
        """Read queue intake, current jobs, and service latency from Go."""
        if self.engine_mode != "service":
            raise EngineError([], 503, "", "the live monitor requires service mode")
        params = {"limit": str(limit)}
        if status:
            params["status"] = status
        if kind:
            params["kind"] = kind
        if page is not None:
            params["page"] = str(page)
        if page_size is not None:
            params["page_size"] = str(page_size)
        if request_page is not None:
            params["request_page"] = str(request_page)
        if request_page_size is not None:
            params["request_page_size"] = str(request_page_size)
        return await self._service_get("/internal/v1/monitor", params)

    async def get_runtime_settings(self) -> dict:
        if self.engine_mode != "service":
            raise EngineError([], 503, "", "runtime settings require service mode")
        return await self._service_get("/internal/v1/settings")

    async def update_runtime_settings(self, payload: dict[str, Any]) -> dict:
        return await self._service_patch("/internal/v1/settings", payload)

    @staticmethod
    def _normalize_service_account(payload: dict[str, Any]) -> dict[str, Any]:
        """Translate the legacy cookie-file selector into a registered account ID."""
        normalized = dict(payload)
        cookies = normalized.pop("cookies", None)
        account_id = str(normalized.get("account_id") or "").strip()
        if account_id and not all(char.isalnum() or char in "-_" for char in account_id):
            raise EngineError(["service", "select-account"], 422, "", "account_id contains unsupported characters")
        if cookies:
            name = Path(str(cookies)).name
            if not (name.startswith("account_") and name.endswith(".json")):
                raise EngineError(["service", "select-account"], 422, "", "cookies must name a registered account_<key>.json bundle")
            legacy_id = name[len("account_"):-len(".json")]
            if not legacy_id:
                raise EngineError(["service", "select-account"], 422, "", "cookie bundle has an empty account id")
            if account_id and account_id != legacy_id:
                raise EngineError(["service", "select-account"], 409, "", "account_id does not match the legacy cookie bundle selector")
            account_id = legacy_id
        if account_id:
            normalized["account_id"] = account_id
        return normalized

    def _subprocess_cookie_selector(
        self, cookies: Optional[str], account_id: Optional[str]
    ) -> Optional[str]:
        if not account_id:
            return cookies
        account_id = account_id.strip()
        if not account_id or not all(char.isalnum() or char in "-_" for char in account_id):
            raise EngineError(["flow", "select-account"], 422, "", "account_id contains unsupported characters")
        expected = f"account_{account_id}.json"
        if cookies and Path(cookies).name != expected:
            raise EngineError(["flow", "select-account"], 409, "", "account_id does not match the cookie bundle selector")
        return str(self.root / "cookies" / expected)

    async def get_projects(
        self, cookies: Optional[str] = None, account_id: Optional[str] = None
    ) -> list[dict]:
        """List the account's Flow projects.

        `--json` is not optional: without it the command renders a table, and a
        table is not something this can read. The flag yields a **bare array**,
        not an object wrapping one.
        """
        if self.engine_mode == "service":
            params: dict[str, Any] = {}
            selector = self._normalize_service_account({"cookies": cookies, "account_id": account_id})
            if selector.get("account_id"):
                params["account_id"] = selector["account_id"]
            result = await self._service_get("/internal/v1/projects", params)
            return result if isinstance(result, list) else []

        cookies = self._subprocess_cookie_selector(cookies, account_id)
        args: list[Any] = ["projects", "--json"]
        if cookies:
            args += ["--cookies", cookies]
        out, _ = await self._run(*args)
        try:
            parsed = extract_json(out)
        except ValueError:
            return []
        if isinstance(parsed, list):
            return parsed
        projects = parsed.get("projects") if isinstance(parsed, dict) else None
        return projects if isinstance(projects, list) else []

    # -- database ---------------------------------------------------------

    def _connect(self) -> sqlite3.Connection:
        """Open the database read-only.

        Read-only matters twice: it cannot create a database that is not there,
        and it cannot take a write lock on one the engine is using. `mode=ro`
        plus a busy timeout is what keeps a concurrent generation from making a
        status read fail.
        """
        return sqlite3.connect(
            f"file:{self.db_path}?mode=ro", uri=True, timeout=5
        )

    def _read_balance(self) -> dict:
        if not self.db_path.exists():
            return {"accounts": [], "total_credits": 0, "unread": 0}

        with self._connect() as conn:
            rows = conn.execute(
                "select account_id, credits, status from accounts order by account_id"
            ).fetchall()

        accounts = [
            {"account_id": account_id, "credits": credits, "status": status or "unknown"}
            for account_id, credits, status in rows
        ]
        # A null balance is "nobody has read it", not zero — the store keeps the
        # two apart on purpose, and folding null in as zero would understate the
        # total by exactly the accounts that have never been checked.
        read = [a for a in accounts if a["credits"] is not None]

        return {
            "accounts": accounts,
            "total_credits": sum(a["credits"] for a in read),
            "unread": len(accounts) - len(read),
        }

    def _read_stats(self) -> dict:
        if not self.db_path.exists():
            return {"generations": [], "totals": {}}

        with self._connect() as conn:
            rows = conn.execute(
                "select job_id, account_id, kind, model, aspect, status, "
                "       credits_spent, elapsed_ms, created_at "
                "from generations order by id desc limit 50"
            ).fetchall()
            by_status = dict(
                conn.execute("select status, count(*) from generations group by status")
            )
            by_kind = dict(
                conn.execute("select kind, count(*) from generations group by kind")
            )
            media_count = conn.execute("select count(*) from media").fetchone()[0]
            # Summed in SQL so a NULL from a job that never finished is skipped
            # rather than counted as free.
            credits_spent = conn.execute(
                "select coalesce(sum(credits_spent), 0) from generations"
            ).fetchone()[0]

        return {
            "generations": [
                {
                    "job_id": job_id,
                    "account_id": account_id,
                    "kind": kind,
                    "model": model,
                    "aspect": aspect,
                    "status": status,
                    "credits_spent": credits_spent,
                    "elapsed_seconds": (elapsed_ms / 1000) if elapsed_ms else None,
                    "created_at": created_at,
                }
                for job_id, account_id, kind, model, aspect, status,
                    credits_spent, elapsed_ms, created_at in rows
            ],
            "totals": {
                "generations": sum(by_status.values()),
                "by_status": by_status,
                "by_kind": by_kind,
                "media_files": media_count,
                "credits_spent": credits_spent,
            },
        }

    # -- misc -------------------------------------------------------------

    def available(self) -> bool:
        """Whether the engine binary is where this expects it."""
        if self.engine_mode == "service":
            return bool(self.service_token)
        return self.binary.exists()
