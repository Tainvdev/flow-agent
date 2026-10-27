from __future__ import annotations

from typing import Any, AsyncIterator

import httpx


class NodeClientError(RuntimeError):
    def __init__(self, node_id: str, status_code: int, detail: str, *, ambiguous: bool = False) -> None:
        self.node_id = node_id
        self.status_code = status_code
        # A transport failure or malformed response does not tell us whether
        # the node persisted the job before the acknowledgement was lost.
        self.ambiguous = ambiguous
        super().__init__(detail)


class NodeClient:
    """Authenticated HTTP client for one external Go service."""

    def __init__(self, node_id: str, base_url: str, token: str, timeout: float = 15.0) -> None:
        self.node_id = node_id
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.timeout = timeout
        self.http: httpx.AsyncClient | None = None

    async def _client(self) -> httpx.AsyncClient:
        if self.http is None:
            self.http = httpx.AsyncClient(
                timeout=httpx.Timeout(self.timeout, connect=min(5.0, self.timeout)),
                limits=httpx.Limits(max_connections=100, max_keepalive_connections=20),
            )
        return self.http

    async def close(self) -> None:
        if self.http is not None:
            await self.http.aclose()
            self.http = None

    async def request_json(self, method: str, path: str, **kwargs: Any) -> dict[str, Any]:
        client = await self._client()
        headers = dict(kwargs.pop("headers", {}))
        headers["Authorization"] = f"Bearer {self.token}"
        try:
            response = await client.request(method, f"{self.base_url}{path}", headers=headers, **kwargs)
        except httpx.HTTPError as exc:
            raise NodeClientError(self.node_id, 503, str(exc), ambiguous=True) from exc
        if response.status_code >= 400:
            raise NodeClientError(self.node_id, response.status_code, response.text[:2000])
        try:
            value = response.json()
        except ValueError as exc:
            raise NodeClientError(self.node_id, 502, "node returned invalid JSON", ambiguous=True) from exc
        if not isinstance(value, dict):
            raise NodeClientError(self.node_id, 502, "node returned a non-object response")
        return value

    async def submit(self, kind: str, payload: dict[str, Any], idempotency_key: str) -> dict[str, Any]:
        return await self.request_json(
            "POST", "/internal/v1/jobs",
            headers={"Idempotency-Key": idempotency_key},
            json={"kind": kind, "payload": payload, "payload_version": 1},
        )

    async def upload(self, file_path: str, *, account_id: str = "", force: bool = False,
                     idempotency_key: str) -> dict[str, Any]:
        client = await self._client()
        try:
            with open(file_path, "rb") as stream:
                response = await client.post(
                    f"{self.base_url}/internal/v1/uploads",
                    headers={"Authorization": f"Bearer {self.token}", "Idempotency-Key": idempotency_key},
                    data={"account_id": account_id, "force": str(force).lower()},
                    files={"file": (file_path, stream, "video/mp4")},
                )
        except (OSError, httpx.HTTPError) as exc:
            raise NodeClientError(self.node_id, 503, str(exc)) from exc
        if response.status_code >= 400:
            raise NodeClientError(self.node_id, response.status_code, response.text[:2000])
        try:
            value = response.json()
        except ValueError as exc:
            raise NodeClientError(self.node_id, 502, "node returned invalid JSON") from exc
        if not isinstance(value, dict):
            raise NodeClientError(self.node_id, 502, "node returned a non-object response")
        return value

    async def by_key(self, key: str) -> dict[str, Any] | None:
        try:
            return await self.request_json("GET", "/internal/v1/jobs/by-key", params={"key": key})
        except NodeClientError as exc:
            if exc.status_code == 404:
                return None
            raise

    async def job(self, node_job_id: str) -> dict[str, Any]:
        return await self.request_json("GET", f"/internal/v1/jobs/{node_job_id}")

    async def cancel(self, node_job_id: str) -> None:
        await self.request_json("POST", f"/internal/v1/jobs/{node_job_id}/cancel")

    async def heartbeat(self) -> dict[str, Any]:
        return await self.request_json("GET", "/internal/v1/node")

    async def projects(self, account_id: str = "") -> list[dict[str, Any]]:
        client = await self._client()
        try:
            response = await client.get(
                f"{self.base_url}/internal/v1/projects",
                headers={"Authorization": f"Bearer {self.token}"},
                params={"account_id": account_id},
            )
        except httpx.HTTPError as exc:
            raise NodeClientError(self.node_id, 503, str(exc)) from exc
        if response.status_code >= 400:
            raise NodeClientError(self.node_id, response.status_code, response.text[:2000])
        try:
            value = response.json()
        except ValueError as exc:
            raise NodeClientError(self.node_id, 502, "node returned invalid JSON") from exc
        projects = value.get("projects", value) if isinstance(value, dict) else value
        return projects if isinstance(projects, list) else []

    async def stream_media(self, media_key: str, headers: dict[str, str] | None = None) -> httpx.Response:
        client = await self._client()
        outgoing = {"Authorization": f"Bearer {self.token}", **(headers or {})}
        try:
            response = await client.send(
                client.build_request("GET", f"{self.base_url}/internal/v1/media/{media_key}", headers=outgoing),
                stream=True,
            )
        except httpx.HTTPError as exc:
            raise NodeClientError(self.node_id, 503, str(exc)) from exc
        if response.status_code >= 400:
            detail = (await response.aread()).decode("utf-8", "replace")
            await response.aclose()
            raise NodeClientError(self.node_id, response.status_code, detail[:2000])
        return response
