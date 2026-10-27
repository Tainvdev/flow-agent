from __future__ import annotations

import asyncio
import json
import os
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, AsyncIterator, Optional

from sqlalchemy import text

from .errors import EngineError
from .db import ControlPlane
from .models import RoutedJob
from .node_client import NodeClient, NodeClientError
from .registry import NodeRegistry
from .router import RoutingError, StickyRouter


TERMINAL = {"succeeded", "failed", "cancelled", "partial", "submission_unknown",
            "dispatch_unknown", "empty", "ready"}


class GatewayEngine:
    """Public API adapter backed by the PostgreSQL routing control plane."""

    def __init__(self, db: ControlPlane | None = None) -> None:
        self.control_plane = db or ControlPlane()
        self.registry = NodeRegistry(
            self.control_plane,
            int(os.getenv("FLOW_NODE_HEARTBEAT_TTL", "45")),
        )
        self.router = StickyRouter()
        self.clients: dict[str, NodeClient] = {}
        self.engine_mode = "gateway"
        self.service_url = ""
        self.output_dir = None
        self.binary = None

    async def aclose(self) -> None:
        for client in self.clients.values():
            await client.close()
        self.clients.clear()

    def _client(self, node_id: str, base_url: str) -> NodeClient:
        client = self.clients.get(node_id)
        if client is None or client.base_url != base_url:
            token = os.getenv("FLOW_NODE_TOKEN", os.getenv("FLOW_GATEWAY_SECRET", ""))
            token_map = os.getenv("FLOW_NODE_TOKENS", "")
            if token_map:
                try:
                    token = str(json.loads(token_map).get(node_id, token))
                except json.JSONDecodeError:
                    pass
            client = NodeClient(node_id, base_url, token, float(os.getenv("FLOW_NODE_REQUEST_TIMEOUT", "15")))
            self.clients[node_id] = client
        return client

    async def _node_for(self, node_id: str):
        node = await self.registry.get(node_id)
        if node is None:
            raise EngineError(["gateway", node_id], 503, "node is not registered")
        return node

    async def enqueue_service_job(self, kind: str, payload: dict[str, Any], idempotency_key: Optional[str] = None) -> dict:
        if self.control_plane.sessions is None:
            raise EngineError(["gateway", "submit"], 503, "control plane is unavailable")
        key = idempotency_key or str(uuid.uuid4())
        async with self.control_plane.sessions() as read_session:
            existing = (await read_session.execute(text(
                "SELECT job_id, status, node_id, node_job_id FROM gateway_jobs WHERE idempotency_key=:key"
            ), {"key": key})).mappings().one_or_none()
        if existing:
            return {"job_id": str(existing["job_id"]), "status": existing["status"], "node_id": existing["node_id"]}

        payload = {key: value for key, value in payload.items()
                   if key not in {"account_id", "all_accounts"}}
        nodes = await self.registry.all()
        try:
            node = self.router.choose(nodes, kind=kind)
        except RoutingError as exc:
            raise EngineError(["gateway", "route", kind], 503, str(exc)) from exc

        job_id = uuid.uuid4()
        async with self.control_plane.sessions.begin() as session:
            # The unique idempotency constraint is the final race guard between
            # two Gateway workers receiving the same client retry concurrently.
            await session.execute(text("""
                INSERT INTO gateway_jobs(job_id, kind, payload, idempotency_key,
                                         requested_account, node_id, status, routing_reason)
                VALUES (:job_id, :kind, CAST(:payload AS jsonb), :key,
                        '', :node_id, 'routing', 'least_loaded')
                ON CONFLICT (idempotency_key) DO NOTHING
            """), {
                "job_id": job_id, "kind": kind, "payload": json.dumps(payload),
                "key": key, "node_id": node.node_id,
            })
            row = (await session.execute(text(
                "SELECT job_id, status, node_id FROM gateway_jobs WHERE idempotency_key=:key"
            ), {"key": key})).mappings().one()
            job_id = row["job_id"]
            if row["status"] != "routing":
                return {"job_id": str(job_id), "status": row["status"], "node_id": row["node_id"]}

        node_key = f"gateway:{key}"
        attempted_nodes: set[str] = set()
        max_attempts = max(1, int(os.getenv("FLOW_GATEWAY_MAX_DISPATCH_ATTEMPTS", "3")))
        retry_backoff = max(0.0, float(os.getenv("FLOW_GATEWAY_DISPATCH_RETRY_BACKOFF_SECONDS", "0")))

        for attempt in range(1, max_attempts + 1):
            attempted_nodes.add(node.node_id)
            async with self.control_plane.sessions.begin() as session:
                await session.execute(text("""
                    UPDATE gateway_jobs SET node_id=:node_id, updated_at=now()
                    WHERE job_id=:job_id AND status='routing'
                """), {"job_id": job_id, "node_id": node.node_id})
                await session.execute(text("""
                    INSERT INTO dispatch_attempts(job_id, node_id, attempt_no, outcome)
                    VALUES (:job_id, :node_id, :attempt_no, 'started')
                """), {"job_id": job_id, "node_id": node.node_id, "attempt_no": attempt})

            client = self._client(node.node_id, node.advertise_url)
            try:
                accepted = await client.submit(kind, payload, node_key)
            except NodeClientError as exc:
                # The caller explicitly prefers availability over exactly-once
                # execution. Retry operational failures on another node; keep
                # validation/authentication failures local because another node
                # cannot make the same invalid request valid.
                failover_eligible = exc.status_code not in {400, 401, 403, 404, 422}
                if failover_eligible and attempt < max_attempts:
                    async with self.control_plane.sessions.begin() as session:
                        await session.execute(text("""
                            UPDATE dispatch_attempts SET outcome='retrying_unknown', http_status=:code,
                                error=:error, finished_at=now()
                            WHERE job_id=:job_id AND attempt_no=:attempt_no
                        """), {"job_id": job_id, "attempt_no": attempt,
                              "code": exc.status_code, "error": str(exc)})
                        await session.execute(text("""
                            INSERT INTO job_events(job_id, event_type, data)
                            VALUES (:job_id, 'dispatch_retry', CAST(:data AS jsonb))
                        """), {"job_id": job_id, "data": json.dumps({
                            "attempt": attempt, "node_id": node.node_id,
                            "reason": str(exc), "http_status": exc.status_code,
                        })})
                    if retry_backoff:
                        await asyncio.sleep(retry_backoff * attempt)
                    try:
                        node = self.router.choose(
                            await self.registry.all(), kind=kind,
                            excluded_node_ids=attempted_nodes,
                        )
                    except RoutingError as route_exc:
                        async with self.control_plane.sessions.begin() as session:
                            await session.execute(text("""
                                UPDATE gateway_jobs SET status='failed', error=:error,
                                       error_code=503, updated_at=now()
                                WHERE job_id=:job_id AND status='routing'
                            """), {"job_id": job_id, "error": str(route_exc)})
                        raise EngineError(["gateway", "route", kind], 503, str(route_exc),
                                          job_id=str(job_id), idempotency_key=key) from route_exc
                    continue

                status = "dispatch_unknown" if exc.ambiguous else "failed"
                outcome = "unknown" if exc.ambiguous else "failed"
                async with self.control_plane.sessions.begin() as session:
                    await session.execute(text("""
                        UPDATE dispatch_attempts SET outcome=:outcome, http_status=:code,
                            error=:error, finished_at=now()
                        WHERE job_id=:job_id AND attempt_no=:attempt_no
                    """), {"job_id": job_id, "attempt_no": attempt, "outcome": outcome,
                          "code": exc.status_code, "error": str(exc)})
                    await session.execute(text("""
                        UPDATE gateway_jobs SET status=:status, error=:error,
                               error_code=:code, updated_at=now()
                        WHERE job_id=:job_id AND status='routing'
                    """), {"job_id": job_id, "status": status,
                          "error": str(exc), "code": exc.status_code})
                raise EngineError([node.advertise_url, kind], exc.status_code, "", str(exc),
                                  job_id=str(job_id), idempotency_key=key) from exc

            node_job_id = str(accepted.get("job_id") or "")
            if not node_job_id:
                error = "node returned no job_id; dispatch outcome is unknown"
                if attempt < max_attempts:
                    async with self.control_plane.sessions.begin() as session:
                        await session.execute(text("""
                            UPDATE dispatch_attempts SET outcome='retrying_unknown', http_status=502,
                                error=:error, finished_at=now() WHERE job_id=:job_id AND attempt_no=:attempt_no
                        """), {"job_id": job_id, "attempt_no": attempt, "error": error})
                    try:
                        node = self.router.choose(
                            await self.registry.all(), kind=kind,
                            excluded_node_ids=attempted_nodes,
                        )
                    except RoutingError as route_exc:
                        pass
                    else:
                        continue
                async with self.control_plane.sessions.begin() as session:
                    await session.execute(text("""
                        UPDATE dispatch_attempts SET outcome='unknown', http_status=502,
                            error=:error, finished_at=now()
                        WHERE job_id=:job_id AND attempt_no=:attempt_no
                    """), {"job_id": job_id, "attempt_no": attempt, "error": error})
                    await session.execute(text("""
                        UPDATE gateway_jobs SET status='dispatch_unknown', error=:error,
                               error_code=502, updated_at=now()
                        WHERE job_id=:job_id AND status='routing'
                    """), {"job_id": job_id, "error": error})
                raise EngineError([node.advertise_url, kind], 502, "", error,
                                  job_id=str(job_id), idempotency_key=key)

            # From this point the job is owned by this node permanently. Never
            # retry or move it after an accepted acknowledgement.
            async with self.control_plane.sessions.begin() as session:
                await session.execute(text("""
                    UPDATE dispatch_attempts SET outcome='accepted', http_status=202,
                        finished_at=now() WHERE job_id=:job_id AND attempt_no=:attempt_no
                """), {"job_id": job_id, "attempt_no": attempt})
                await session.execute(text("""
                    UPDATE gateway_jobs SET node_job_id=:node_job_id, status=:status,
                           accepted_at=now(), updated_at=now() WHERE job_id=:job_id
                """), {"job_id": job_id, "node_job_id": node_job_id,
                      "status": accepted.get("status", "queued")})
                await session.execute(text("""
                    INSERT INTO job_events(job_id, event_type, data)
                    VALUES (:job_id, 'status', CAST(:data AS jsonb))
                """), {"job_id": job_id, "data": json.dumps({"status": accepted.get("status", "queued")})})
            return {"job_id": str(job_id), "status": accepted.get("status", "queued"), "node_id": node.node_id}

        raise EngineError(["gateway", "dispatch", kind], 503, "dispatch attempts exhausted",
                          job_id=str(job_id), idempotency_key=key)

    async def get_service_job(self, job_id: str) -> dict:
        row = await self._job_row(job_id)
        if row is None:
            raise EngineError(["gateway", "job", job_id], 404, "job not found", job_id=job_id)
        await self._refresh(row)
        row = await self._job_row(job_id) or row
        failover_budget = max(1, int(os.getenv("FLOW_GATEWAY_MAX_DISPATCH_ATTEMPTS", "3")))
        for _ in range(failover_budget):
            if row.get("status") not in {"submission_unknown", "dispatch_unknown"}:
                break
            retried = await self._retry_unknown_job(row)
            row = await self._job_row(job_id) or row
            if not retried:
                break
        if row.get("status") == "waiting_children":
            await self._refresh_parent(row)
        row = await self._job_row(job_id) or row
        return _public_job(row)

    async def _retry_unknown_job(self, row: dict[str, Any]) -> bool:
        """Fail over an unresolved dispatch, accepting duplicate risk by policy."""
        if self.control_plane.sessions is None:
            return False
        max_attempts = max(1, int(os.getenv("FLOW_GATEWAY_MAX_DISPATCH_ATTEMPTS", "3")))
        async with self.control_plane.sessions() as session:
            attempts = (await session.execute(text("""
                SELECT attempt_no, node_id FROM dispatch_attempts
                WHERE job_id=:job_id ORDER BY attempt_no
            """), {"job_id": row["job_id"]})).mappings().all()
        if len(attempts) >= max_attempts:
            return False
        excluded = {str(item["node_id"]) for item in attempts}
        try:
            node = self.router.choose(
                await self.registry.all(), kind=str(row["kind"]),
                excluded_node_ids=excluded,
            )
        except RoutingError:
            return False
        raw_payload = row.get("payload") or {}
        if isinstance(raw_payload, dict):
            payload = raw_payload
        else:
            try:
                payload = json.loads(raw_payload)
            except (TypeError, json.JSONDecodeError):
                return False
        if not isinstance(payload, dict):
            return False
        attempt_no = len(attempts) + 1
        async with self.control_plane.sessions.begin() as session:
            claimed = await session.execute(text("""
                UPDATE gateway_jobs SET status='routing', node_id=:node_id,
                    node_job_id=NULL, error='', error_code=0, updated_at=now()
                WHERE job_id=:job_id AND status IN ('submission_unknown', 'dispatch_unknown')
            """), {"job_id": row["job_id"], "node_id": node.node_id})
            if not claimed.rowcount:
                return False
            await session.execute(text("""
                INSERT INTO dispatch_attempts(job_id, node_id, attempt_no, outcome)
                VALUES (:job_id, :node_id, :attempt_no, 'started')
            """), {"job_id": row["job_id"], "node_id": node.node_id, "attempt_no": attempt_no})
        key = str(row.get("idempotency_key") or f"gateway:{row['job_id']}")
        try:
            if str(row["kind"]) == "upload-video":
                source_path = str(payload.get("source_path") or "")
                if not source_path or not Path(source_path).is_file():
                    raise NodeClientError(node.node_id, 422, "gateway upload source is no longer available")
                accepted = await self._client(node.node_id, node.advertise_url).upload(
                    source_path, force=bool(payload.get("force", False)), idempotency_key=f"gateway:{key}")
            else:
                accepted = await self._client(node.node_id, node.advertise_url).submit(
                    str(row["kind"]), payload, f"gateway:{key}")
        except NodeClientError as exc:
            status = "dispatch_unknown" if exc.ambiguous else "failed"
            outcome = "unknown" if exc.ambiguous else "failed"
            async with self.control_plane.sessions.begin() as session:
                await session.execute(text("""
                    UPDATE dispatch_attempts SET outcome=:outcome, http_status=:code,
                        error=:error, finished_at=now()
                    WHERE job_id=:job_id AND attempt_no=:attempt_no
                """), {"job_id": row["job_id"], "attempt_no": attempt_no,
                      "outcome": outcome, "code": exc.status_code, "error": str(exc)})
                await session.execute(text("""
                    UPDATE gateway_jobs SET status=:status, error=:error,
                        error_code=:code, updated_at=now() WHERE job_id=:job_id
                """), {"job_id": row["job_id"], "status": status,
                      "error": str(exc), "code": exc.status_code})
            return True
        node_job_id = str(accepted.get("job_id") or "")
        if not node_job_id:
            error = "failover node returned no job_id; dispatch outcome is unknown"
            async with self.control_plane.sessions.begin() as session:
                await session.execute(text("""
                    UPDATE dispatch_attempts SET outcome='unknown', http_status=502,
                        error=:error, finished_at=now() WHERE job_id=:job_id AND attempt_no=:attempt_no
                """), {"job_id": row["job_id"], "attempt_no": attempt_no, "error": error})
                await session.execute(text("""
                    UPDATE gateway_jobs SET status='dispatch_unknown', error=:error,
                        error_code=502, updated_at=now() WHERE job_id=:job_id
                """), {"job_id": row["job_id"], "error": error})
            return True
        async with self.control_plane.sessions.begin() as session:
            await session.execute(text("""
                UPDATE dispatch_attempts SET outcome='accepted', http_status=202,
                    finished_at=now() WHERE job_id=:job_id AND attempt_no=:attempt_no
            """), {"job_id": row["job_id"], "attempt_no": attempt_no})
            await session.execute(text("""
                UPDATE gateway_jobs SET node_job_id=:node_job_id, status=:status,
                    accepted_at=now(), updated_at=now(), error='', error_code=0
                WHERE job_id=:job_id
            """), {"job_id": row["job_id"], "node_job_id": node_job_id,
                  "status": accepted.get("status", "queued")})
            await session.execute(text("""
                INSERT INTO job_events(job_id, event_type, data)
                VALUES (:job_id, 'failover_accepted', CAST(:data AS jsonb))
            """), {"job_id": row["job_id"], "data": json.dumps({
                "from_node": row.get("node_id"), "to_node": node.node_id,
                "attempt": attempt_no,
            })})
        return True

    async def _refresh_parent(self, row: dict[str, Any]) -> None:
        if self.control_plane.sessions is None:
            return
        async with self.control_plane.sessions() as session:
            children = (await session.execute(text("""
                SELECT j.status, j.result, j.error, j.error_code
                FROM gateway_job_children c JOIN gateway_jobs j ON j.job_id=c.child_job_id
                WHERE c.parent_job_id=:parent_id ORDER BY c.child_index
            """), {"parent_id": row["job_id"]})).mappings().all()
        if not children or not all(item["status"] in TERMINAL for item in children):
            return
        failed = [item for item in children if item["status"] not in {"succeeded", "ready", "empty"}]
        status = "partial" if failed and len(failed) < len(children) else ("failed" if failed else "succeeded")
        result = {"status": status, "results": [item["result"] for item in children if item["result"] is not None]}
        async with self.control_plane.sessions.begin() as session:
            await session.execute(text("""
                UPDATE gateway_jobs SET status=:status, result=CAST(:result AS jsonb),
                    error=:error, error_code=:code, updated_at=now(), finished_at=now()
                WHERE job_id=:job_id AND status='waiting_children'
            """), {"job_id": row["job_id"], "status": status, "result": json.dumps(result),
                  "error": "; ".join(item["error"] or "child failed" for item in failed),
                  "code": int(failed[0]["error_code"] or 502) if failed else 0})

    async def _job_row(self, job_id: str) -> dict[str, Any] | None:
        if self.control_plane.sessions is None:
            raise EngineError(["gateway", "job", job_id], 503, "control plane is unavailable")
        try:
            parsed = uuid.UUID(job_id)
        except ValueError:
            return None
        async with self.control_plane.sessions() as session:
            row = (await session.execute(text("""
                SELECT job_id, kind, payload, status, result, error, error_code,
                       node_id, node_job_id, idempotency_key, created_at, updated_at,
                       accepted_at, finished_at,
                       (SELECT COUNT(*) FROM dispatch_attempts a WHERE a.job_id=gateway_jobs.job_id) AS attempts
                FROM gateway_jobs WHERE job_id=:job_id
            """), {"job_id": parsed})).mappings().one_or_none()
            if row is None:
                return None
            value = dict(row)
            events = (await session.execute(text("""
                SELECT event_id, event_type, data, occurred_at
                FROM job_events WHERE job_id=:job_id
                ORDER BY event_id ASC LIMIT 200
            """), {"job_id": parsed})).mappings().all()
            value["events"] = [dict(event) for event in events]
            return value

    async def _refresh(self, row: dict[str, Any]) -> None:
        if row.get("node_id") is None or row.get("node_job_id") is None:
            return
        node = await self._node_for(row["node_id"])
        client = self._client(node.node_id, node.advertise_url)
        try:
            remote = await client.job(row["node_job_id"])
        except NodeClientError:
            return
        status = str(remote.get("status", row["status"]))
        result = self._normalize_media_result(remote.get("result"))
        error = str(remote.get("error", "") or "")
        async with self.control_plane.sessions.begin() as session:  # type: ignore[union-attr]
            await session.execute(text("""
                UPDATE gateway_jobs SET status=:status, result=CAST(:result AS jsonb),
                       error=:error, error_code=:code, updated_at=now(),
                       finished_at=CASE WHEN :terminal THEN COALESCE(finished_at, now()) ELSE finished_at END
                WHERE job_id=:job_id
            """), {"job_id": row["job_id"], "status": status,
                  "result": json.dumps(result) if result is not None else "null",
                  "error": error, "code": int(remote.get("error_code", 0) or 0),
                  "terminal": status in TERMINAL})
            if status != row["status"]:
                await session.execute(text("""
                    INSERT INTO job_events(job_id, event_type, data)
                    VALUES (:job_id, 'status', CAST(:data AS jsonb))
                """), {"job_id": row["job_id"], "data": json.dumps({"status": status})})
            if status in TERMINAL:
                await self._record_media_routes(session, result, row)

    async def _record_media_routes(self, session: Any, result: Any, row: dict[str, Any]) -> None:
        if not isinstance(result, dict):
            return
        media_items = result.get("media")
        if not isinstance(media_items, list):
            return
        for item in media_items:
            if not isinstance(item, dict) or not item.get("media_id"):
                continue
            key = str(item.get("file_name") or item["media_id"])
            await session.execute(text("""
                INSERT INTO media_routes(media_key, job_id, node_id, node_media_id,
                                         account_id, project_id, file_name, content_type, bytes)
                VALUES (:key, :job_id, :node_id, :media_id, :account_id, :project_id,
                        :file_name, :content_type, :bytes)
                ON CONFLICT(media_key) DO UPDATE SET
                    job_id=excluded.job_id, node_id=excluded.node_id,
                    node_media_id=excluded.node_media_id, file_name=excluded.file_name,
                    content_type=excluded.content_type, bytes=excluded.bytes
            """), {
                "key": key, "job_id": row["job_id"], "node_id": row["node_id"],
                "media_id": str(item["media_id"]), "account_id": str(item.get("account_id", "")),
                "project_id": str(item.get("project_id", "")), "file_name": key,
                "content_type": "video/mp4" if key.lower().endswith(".mp4") else "image/jpeg",
                "bytes": item.get("bytes"),
            })

    @staticmethod
    def _normalize_media_result(result: Any) -> Any:
        if not isinstance(result, dict):
            return result
        normalized = json.loads(json.dumps(result))
        media_items = normalized.get("media")
        if isinstance(media_items, list):
            for item in media_items:
                if not isinstance(item, dict) or not item.get("media_id"):
                    continue
                key = str(item.get("file_name") or item["media_id"])
                item["url"] = f"/api/v1/media/{key}"
                item.pop("file_path", None)
        files = normalized.get("files")
        if isinstance(files, list):
            normalized["files"] = [
                f"/api/v1/media/{str(item).rsplit('/', 1)[-1]}" if isinstance(item, str) else item
                for item in files
            ]
        return normalized

    async def wait_job(self, job_id: str, timeout: int = 900) -> dict:
        deadline = asyncio.get_running_loop().time() + timeout
        while True:
            value = await self.get_service_job(job_id)
            if value.get("status") in TERMINAL:
                if value.get("status") in {"failed", "submission_unknown", "dispatch_unknown", "cancelled"}:
                    raise EngineError(["gateway", "job", job_id], int(value.get("error_code") or 502), "", value.get("error") or "job failed", job_id=job_id)
                return value.get("result") or value
            if asyncio.get_running_loop().time() >= deadline:
                raise EngineError(["gateway", "job", job_id], -1, "", "job is still running", job_id=job_id)
            await asyncio.sleep(0.5)

    async def generate_image(self, prompt: str, aspect: str = "1:1", count: int = 1,
                             model: str = "narwhal",
                             cookies: str | None = None, download: bool = True,
                             timeout: int | None = None, idempotency_key: str | None = None,
                             reference_data: list[dict[str, str]] | None = None) -> dict:
        payload: dict[str, Any] = {"prompt": prompt, "aspect": aspect, "count": count, "model": model,
                                   "no_download": not download}
        if reference_data: payload["reference_data"] = reference_data
        accepted = await self.enqueue_service_job("image", payload, idempotency_key)
        return await self.wait_job(accepted["job_id"], timeout or 900)

    async def generate_video(self, prompt: str, aspect: str = "landscape", duration: str = "8s",
                             quality: str = "720p", count: int = 1, start_image: str | None = None,
                             cookies: str | None = None, download: bool = True,
                             timeout: int | None = None, idempotency_key: str | None = None,
                             reference_data: list[dict[str, str]] | None = None,
                             model: str | None = None) -> dict:
        payload: dict[str, Any] = {"prompt": prompt, "aspect": aspect, "duration": duration,
                                   "quality": quality, "count": count,
                                   "no_download": not download}
        for key, value in (("start_image", start_image), ("model", model), ("reference_data", reference_data)):
            if value: payload[key] = value
        accepted = await self.enqueue_service_job("video", payload, idempotency_key)
        return await self.wait_job(accepted["job_id"], timeout or 900)

    async def upload_video(self, file_path: str, cookies: str | None = None, force: bool = False,
                           timeout: int | None = None, idempotency_key: str | None = None) -> dict:
        if self.control_plane.sessions is None:
            raise EngineError(["gateway", "upload-video"], 503, "control plane is unavailable")
        if not Path(file_path).is_file():
            raise EngineError(["gateway", "upload-video"], 422, "upload file is not available on the Gateway")
        key = idempotency_key or str(uuid.uuid4())
        async with self.control_plane.sessions() as session:
            existing = (await session.execute(text(
                "SELECT job_id, status FROM gateway_jobs WHERE idempotency_key=:key"
            ), {"key": key})).mappings().one_or_none()
        if existing:
            return await self.wait_job(str(existing["job_id"]), timeout or 900)
        nodes = await self.registry.all()
        try:
            node = self.router.choose(nodes, kind="upload-video")
        except RoutingError as exc:
            raise EngineError(["gateway", "route", "upload-video"], 503, str(exc)) from exc
        job_id = uuid.uuid4()
        payload = {"file_name": Path(file_path).name, "source_path": file_path, "force": force}
        async with self.control_plane.sessions.begin() as session:
            await session.execute(text("""
                INSERT INTO gateway_jobs(job_id, kind, payload, idempotency_key,
                    requested_account, node_id, status, routing_reason)
                VALUES (:job_id, 'upload-video', CAST(:payload AS jsonb), :key,
                    '', :node_id, 'routing', 'upload_stream')
            """), {"job_id": job_id, "payload": json.dumps(payload), "key": key,
                  "node_id": node.node_id})
            await session.execute(text("""
                INSERT INTO dispatch_attempts(job_id, node_id, attempt_no, outcome)
                VALUES (:job_id, :node_id, 1, 'started')
            """), {"job_id": job_id, "node_id": node.node_id})
        attempted_nodes: set[str] = set()
        max_attempts = max(1, int(os.getenv("FLOW_GATEWAY_MAX_DISPATCH_ATTEMPTS", "3")))
        retry_backoff = max(0.0, float(os.getenv("FLOW_GATEWAY_DISPATCH_RETRY_BACKOFF_SECONDS", "0")))
        for attempt in range(1, max_attempts + 1):
            attempted_nodes.add(node.node_id)
            if attempt > 1:
                async with self.control_plane.sessions.begin() as session:
                    await session.execute(text("""
                        UPDATE gateway_jobs SET node_id=:node_id, updated_at=now()
                        WHERE job_id=:job_id AND status='routing'
                    """), {"job_id": job_id, "node_id": node.node_id})
                    await session.execute(text("""
                        INSERT INTO dispatch_attempts(job_id, node_id, attempt_no, outcome)
                        VALUES (:job_id, :node_id, :attempt_no, 'started')
                    """), {"job_id": job_id, "node_id": node.node_id, "attempt_no": attempt})
            try:
                accepted = await self._client(node.node_id, node.advertise_url).upload(
                    file_path, force=force, idempotency_key=f"gateway:{key}")
            except NodeClientError as exc:
                failover_eligible = exc.status_code not in {400, 401, 403, 404, 422}
                if failover_eligible and attempt < max_attempts:
                    async with self.control_plane.sessions.begin() as session:
                        await session.execute(text("""
                            UPDATE dispatch_attempts SET outcome='rejected_retryable', http_status=:code,
                                error=:error, finished_at=now()
                            WHERE job_id=:job_id AND attempt_no=:attempt_no
                        """), {"job_id": job_id, "attempt_no": attempt,
                              "code": exc.status_code, "error": str(exc)})
                    if retry_backoff:
                        await asyncio.sleep(retry_backoff * attempt)
                    try:
                        node = self.router.choose(
                            await self.registry.all(), kind="upload-video",
                            excluded_node_ids=attempted_nodes,
                        )
                    except RoutingError as route_exc:
                        async with self.control_plane.sessions.begin() as session:
                            await session.execute(text("""
                                UPDATE gateway_jobs SET status='failed', error=:error,
                                    error_code=503, updated_at=now() WHERE job_id=:job_id
                            """), {"job_id": job_id, "error": str(route_exc)})
                        raise EngineError(["gateway", "route", "upload-video"], 503, str(route_exc),
                                          job_id=str(job_id)) from route_exc
                    continue
                status = "dispatch_unknown" if exc.ambiguous else "failed"
                outcome = "unknown" if exc.ambiguous else "failed"
                async with self.control_plane.sessions.begin() as session:
                    await session.execute(text("""
                        UPDATE dispatch_attempts SET outcome=:outcome, http_status=:code,
                            error=:error, finished_at=now() WHERE job_id=:job_id AND attempt_no=:attempt_no
                    """), {"job_id": job_id, "attempt_no": attempt, "outcome": outcome,
                          "code": exc.status_code, "error": str(exc)})
                    await session.execute(text("""
                        UPDATE gateway_jobs SET status=:status, error=:error,
                            error_code=:code, updated_at=now() WHERE job_id=:job_id
                    """), {"job_id": job_id, "status": status, "error": str(exc), "code": exc.status_code})
                raise EngineError([node.advertise_url, "upload-video"], exc.status_code, "", str(exc), job_id=str(job_id)) from exc
            node_job_id = str(accepted.get("job_id") or "")
            if not node_job_id:
                error = "node returned no job_id; dispatch outcome is unknown"
                if attempt < max_attempts:
                    async with self.control_plane.sessions.begin() as session:
                        await session.execute(text("""
                            UPDATE dispatch_attempts SET outcome='retrying_unknown', http_status=502,
                                error=:error, finished_at=now() WHERE job_id=:job_id AND attempt_no=:attempt_no
                        """), {"job_id": job_id, "attempt_no": attempt, "error": error})
                    try:
                        node = self.router.choose(
                            await self.registry.all(), kind="upload-video",
                            excluded_node_ids=attempted_nodes,
                        )
                    except RoutingError:
                        pass
                    else:
                        continue
                async with self.control_plane.sessions.begin() as session:
                    await session.execute(text("""
                        UPDATE dispatch_attempts SET outcome='unknown', http_status=502,
                            error=:error, finished_at=now() WHERE job_id=:job_id AND attempt_no=:attempt_no
                    """), {"job_id": job_id, "attempt_no": attempt, "error": error})
                    await session.execute(text("""
                        UPDATE gateway_jobs SET status='dispatch_unknown', error=:error,
                            error_code=502, updated_at=now() WHERE job_id=:job_id
                    """), {"job_id": job_id, "error": error})
                raise EngineError([node.advertise_url, "upload-video"], 502, "", error, job_id=str(job_id))
            async with self.control_plane.sessions.begin() as session:
                await session.execute(text("""
                    UPDATE dispatch_attempts SET outcome='accepted', http_status=202,
                        finished_at=now() WHERE job_id=:job_id AND attempt_no=:attempt_no
                """), {"job_id": job_id, "attempt_no": attempt})
                await session.execute(text("""
                    UPDATE gateway_jobs SET node_job_id=:node_job_id, status=:status,
                        accepted_at=now(), updated_at=now() WHERE job_id=:job_id
                """), {"job_id": job_id, "node_job_id": node_job_id, "status": accepted.get("status", "queued")})
            return await self.wait_job(str(job_id), timeout or 900)
        raise EngineError(["gateway", "dispatch", "upload-video"], 503, "dispatch attempts exhausted", job_id=str(job_id))

    async def service_health(self) -> dict[str, Any]:
        if self.control_plane.sessions is None:
            return {"status": "unavailable", "mode": "gateway"}
        nodes = await self.registry.all()
        return {"status": "ok", "mode": "gateway", "nodes": len(nodes),
                "healthy_nodes": sum(node.healthy for node in nodes)}

    async def service_available(self) -> bool:
        return (await self.service_health()).get("status") == "ok"

    async def get_balance(self, refresh: bool = False) -> dict[str, Any]:
        nodes = await self.registry.all()
        return {"accounts": [
            {"node_id": node.node_id, **account}
            for node in nodes for account in node.accounts.values()
        ]}

    async def get_stats(self) -> dict[str, Any]:
        if self.control_plane.sessions is None:
            raise EngineError(["gateway", "stats"], 503, "control plane is unavailable")
        async with self.control_plane.sessions() as session:
            rows = (await session.execute(text("SELECT status, COUNT(*) AS count FROM gateway_jobs GROUP BY status"))).mappings().all()
        return {"jobs": {row["status"]: row["count"] for row in rows}}

    async def get_monitor(self, **kwargs: Any) -> dict[str, Any]:
        if self.control_plane.sessions is None:
            raise EngineError(["gateway", "monitor"], 503, "control plane is unavailable")
        nodes = await self.registry.all()
        limit = min(max(int(kwargs.get("limit", 50)), 1), 200)
        status = str(kwargs.get("status", "")).strip()
        kind = str(kwargs.get("kind", "")).strip()
        filters = ["1=1"]
        params: dict[str, Any] = {"limit": limit}
        if status:
            filters.append("j.status=:status")
            params["status"] = status
        if kind:
            filters.append("j.kind=:kind")
            params["kind"] = kind
        async with self.control_plane.sessions() as session:
            jobs = (await session.execute(text(f"""
                SELECT j.job_id, j.kind, j.status, j.requested_account,
                       j.node_id, j.node_job_id, j.error, j.error_code,
                       j.created_at, j.updated_at, j.accepted_at, j.finished_at,
                       (SELECT COUNT(*) FROM job_events e WHERE e.job_id=j.job_id) AS event_count,
                       (SELECT COUNT(*) FROM dispatch_attempts a WHERE a.job_id=j.job_id) AS attempt_count
                FROM gateway_jobs j WHERE {' AND '.join(filters)}
                ORDER BY j.created_at DESC LIMIT :limit
            """), params)).mappings().all()
            counts = (await session.execute(text(
                "SELECT status, COUNT(*) AS count FROM gateway_jobs GROUP BY status"
            ))).mappings().all()
        return {
            "nodes": [{"node_id": node.node_id, "advertise_url": node.advertise_url,
                       "status": node.status if node.healthy else "offline",
                       "healthy": node.healthy, "max_jobs": node.max_jobs,
                       "active_jobs": node.active_jobs, "queued_jobs": node.queued_jobs,
                       "accounts": list(node.accounts.values()),
                       "last_heartbeat_at": node.last_heartbeat_at,
                       "lease_expires_at": node.lease_expires_at}
                      for node in nodes],
            "jobs": [dict(job) for job in jobs],
            "job_counts": {row["status"]: row["count"] for row in counts},
        }

    async def get_runtime_settings(self) -> dict[str, Any]:
        return {"values": {}, "defaults": {}, "sources": {}, "scope": "gateway"}

    async def update_runtime_settings(self, payload: dict[str, Any]) -> dict[str, Any]:
        raise EngineError(["gateway", "settings"], 501, "fleet settings endpoint is not implemented yet")

    async def get_projects(self, cookies: str | None = None) -> list[dict[str, Any]]:
        nodes = await self.registry.all()
        if not nodes:
            return []
        node = min(nodes, key=lambda item: item.score())
        try:
            return await self._client(node.node_id, node.advertise_url).projects("")
        except NodeClientError as exc:
            raise EngineError([node.advertise_url, "projects"], exc.status_code, "", str(exc)) from exc

    async def cancel_service_job(self, job_id: str) -> None:
        row = await self._job_row(job_id)
        if row is None:
            raise EngineError(["gateway", "cancel", job_id], 404, "job not found", job_id=job_id)
        if row.get("node_id") and row.get("node_job_id"):
            node = await self._node_for(row["node_id"])
            try:
                await self._client(node.node_id, node.advertise_url).cancel(row["node_job_id"])
            except NodeClientError as exc:
                raise EngineError([node.advertise_url, "cancel"], exc.status_code, "", str(exc), job_id=job_id) from exc

    async def open_media(self, media_key: str, headers: dict[str, str] | None = None):
        if self.control_plane.sessions is None:
            raise EngineError(["gateway", "media", media_key], 503, "control plane is unavailable")
        async with self.control_plane.sessions() as session:
            row = (await session.execute(text("""
                SELECT node_id, node_media_id, file_name, content_type, bytes
                FROM media_routes WHERE media_key=:media_key
            """), {"media_key": media_key})).mappings().one_or_none()
        if row is None:
            raise EngineError(["gateway", "media", media_key], 404, "media not found")
        node = await self._node_for(row["node_id"])
        response = await self._client(node.node_id, node.advertise_url).stream_media(row["node_media_id"], headers)
        return response, row

    async def open_service_job_events(self, job_id: str, last_event_id: str | None = None):
        raise EngineError(["gateway", "events", job_id], 501, "gateway event stream is not wired yet", job_id=job_id)

    async def stream_service_job_events(self, job_id: str, last_event_id: str | None = None) -> AsyncIterator[bytes]:
        """Read the Gateway-owned event cursor; node events are mirrored by refresh."""
        cursor = int(last_event_id or 0)
        while True:
            row = await self._job_row(job_id)
            if row is None:
                raise EngineError(["gateway", "events", job_id], 404, "job not found", job_id=job_id)
            await self._refresh(row)
            row = await self._job_row(job_id) or row
            failover_budget = max(1, int(os.getenv("FLOW_GATEWAY_MAX_DISPATCH_ATTEMPTS", "3")))
            for _ in range(failover_budget):
                if row.get("status") not in {"submission_unknown", "dispatch_unknown"}:
                    break
                retried = await self._retry_unknown_job(row)
                row = await self._job_row(job_id) or row
                if not retried:
                    break
            if row.get("status") == "waiting_children":
                await self._refresh_parent(row)
                row = await self._job_row(job_id) or row
            if self.control_plane.sessions is None:
                raise EngineError(["gateway", "events", job_id], 503, "control plane is unavailable")
            async with self.control_plane.sessions() as session:
                events = (await session.execute(text("""
                    SELECT event_id, event_type, data, occurred_at
                    FROM job_events WHERE job_id=:job_id AND event_id > :cursor
                    ORDER BY event_id LIMIT 100
                """), {"job_id": uuid.UUID(job_id), "cursor": cursor})).mappings().all()
            for event in events:
                cursor = int(event["event_id"])
                payload = json.dumps(event["data"], separators=(",", ":"))
                yield f"id: {cursor}\nevent: {event['event_type']}\ndata: {payload}\n\n".encode()
            if row.get("status") in TERMINAL:
                return
            await asyncio.sleep(0.5)


def _public_job(row: dict[str, Any]) -> dict[str, Any]:
    return {
        "job_id": str(row["job_id"]), "kind": row["kind"], "status": row["status"],
        "node_id": row["node_id"], "attempts": row.get("attempts", 0), "created_at": row["created_at"],
        "updated_at": row["updated_at"], "finished_at": row["finished_at"],
        "result": row["result"], "error": row["error"], "error_code": row["error_code"],
        "events": row.get("events", []),
        "events_url": f"/api/v1/jobs/{row['job_id']}/events",
    }
