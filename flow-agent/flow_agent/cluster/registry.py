from __future__ import annotations

from datetime import datetime, timedelta, timezone
from typing import Any

from sqlalchemy import text

from .db import ControlPlane
from .models import NodeSnapshot


def _utcnow() -> datetime:
    return datetime.now(timezone.utc)


class NodeRegistry:
    def __init__(self, db: ControlPlane, heartbeat_ttl_seconds: int = 45) -> None:
        self.db = db
        self.heartbeat_ttl = heartbeat_ttl_seconds

    async def register(self, payload: dict[str, Any]) -> NodeSnapshot:
        required = ("node_id", "advertise_url")
        missing = [key for key in required if not str(payload.get(key, "")).strip()]
        if missing:
            raise ValueError(f"missing node fields: {', '.join(missing)}")
        node_id = str(payload["node_id"]).strip()
        now = _utcnow()
        lease = now + timedelta(seconds=self.heartbeat_ttl)
        if self.db.sessions is None:
            raise RuntimeError("control plane is not connected")
        async with self.db.sessions.begin() as session:
            await session.execute(text("""
                INSERT INTO nodes(node_id, name, advertise_url, status, version,
                                  capabilities, max_jobs, active_jobs, queued_jobs, metadata,
                                  last_heartbeat_at, lease_expires_at)
                VALUES (:node_id, :name, :url, 'online', :version,
                        CAST(:capabilities AS jsonb), :max_jobs, :active_jobs, :queued_jobs,
                        CAST(:metadata AS jsonb), :heartbeat, :lease)
                ON CONFLICT(node_id) DO UPDATE SET
                    name=excluded.name, advertise_url=excluded.advertise_url,
                    status='online', version=excluded.version,
                    capabilities=excluded.capabilities, max_jobs=excluded.max_jobs,
                    active_jobs=excluded.active_jobs, queued_jobs=excluded.queued_jobs,
                    metadata=excluded.metadata, last_heartbeat_at=excluded.last_heartbeat_at,
                    lease_expires_at=excluded.lease_expires_at
            """), {
                "node_id": node_id,
                "name": str(payload.get("name", "")),
                "url": str(payload["advertise_url"]).rstrip("/"),
                "version": str(payload.get("version", "")),
                "capabilities": _json(payload.get("capabilities", {})),
                "max_jobs": max(1, int(payload.get("max_jobs", 1))),
                "active_jobs": max(0, int(payload.get("active_jobs", 0))),
                "queued_jobs": max(0, int(payload.get("queued_jobs", 0))),
                "metadata": _json(payload.get("metadata", {})),
                "heartbeat": now,
                "lease": lease,
            })
            await self._replace_accounts(session, node_id, payload.get("accounts", []), now)
        return await self.get(node_id)  # type: ignore[return-value]

    async def heartbeat(self, node_id: str, payload: dict[str, Any]) -> NodeSnapshot:
        payload = {**payload, "node_id": node_id}
        if self.db.sessions is None:
            raise RuntimeError("control plane is not connected")
        now = _utcnow()
        lease = now + timedelta(seconds=self.heartbeat_ttl)
        missing = False
        async with self.db.sessions.begin() as session:
            result = await session.execute(text("""
                UPDATE nodes SET name=:name, advertise_url=:url, version=:version,
                    status=CASE WHEN status='draining' THEN 'draining' ELSE 'online' END,
                    capabilities=CAST(:capabilities AS jsonb), max_jobs=:max_jobs,
                    active_jobs=:active_jobs, queued_jobs=:queued_jobs,
                    metadata=CAST(:metadata AS jsonb), last_heartbeat_at=:heartbeat,
                    lease_expires_at=:lease
                WHERE node_id=:node_id
            """), {
                "node_id": node_id,
                "name": str(payload.get("name", "")),
                "url": str(payload.get("advertise_url", "")).rstrip("/"),
                "version": str(payload.get("version", "")),
                "capabilities": _json(payload.get("capabilities", {})),
                "max_jobs": max(1, int(payload.get("max_jobs", 1))),
                "active_jobs": max(0, int(payload.get("active_jobs", 0))),
                "queued_jobs": max(0, int(payload.get("queued_jobs", 0))),
                "metadata": _json(payload.get("metadata", {})),
                "heartbeat": now,
                "lease": lease,
            })
            if not result.rowcount:
                missing = True
            else:
                await self._replace_accounts(session, node_id, payload.get("accounts", []), now)
        if missing:
            return await self.register(payload)
        return await self.get(node_id)  # type: ignore[return-value]

    async def set_status(self, node_id: str, status: str) -> NodeSnapshot | None:
        if status not in {"online", "draining"}:
            raise ValueError("status must be online or draining")
        if self.db.sessions is None:
            raise RuntimeError("control plane is not connected")
        async with self.db.sessions.begin() as session:
            await session.execute(text(
                "UPDATE nodes SET status=:status WHERE node_id=:node_id"
            ), {"node_id": node_id, "status": status})
        return await self.get(node_id)

    async def get(self, node_id: str) -> NodeSnapshot | None:
        if self.db.sessions is None:
            raise RuntimeError("control plane is not connected")
        async with self.db.sessions() as session:
            row = (await session.execute(text("""
                SELECT node_id, advertise_url, status, max_jobs, active_jobs,
                       queued_jobs, capabilities, last_heartbeat_at, lease_expires_at
                FROM nodes WHERE node_id=:node_id
            """), {"node_id": node_id})).mappings().one_or_none()
            if row is None:
                return None
            accounts = (await session.execute(text("""
                SELECT account_id, status, ready, connected, credits, project_id
                FROM node_accounts WHERE node_id=:node_id
            """), {"node_id": node_id})).mappings().all()
        return _snapshot(row, accounts)

    async def all(self) -> list[NodeSnapshot]:
        if self.db.sessions is None:
            raise RuntimeError("control plane is not connected")
        async with self.db.sessions() as session:
            rows = (await session.execute(text("""
                SELECT node_id, advertise_url, status, max_jobs, active_jobs,
                       queued_jobs, capabilities, last_heartbeat_at, lease_expires_at
                FROM nodes ORDER BY node_id
            """))).mappings().all()
            accounts = (await session.execute(text("""
                SELECT node_id, account_id, status, ready, connected, credits, project_id
                FROM node_accounts
            """))).mappings().all()
        by_node: dict[str, list[Any]] = {}
        for account in accounts:
            by_node.setdefault(account["node_id"], []).append(account)
        return [_snapshot(row, by_node.get(row["node_id"], [])) for row in rows]

    async def mark_stale(self) -> int:
        if self.db.sessions is None:
            raise RuntimeError("control plane is not connected")
        async with self.db.sessions.begin() as session:
            result = await session.execute(text("""
                UPDATE nodes SET status='offline'
                WHERE status IN ('online', 'registering')
                  AND (lease_expires_at IS NULL OR lease_expires_at < now())
            """))
            return result.rowcount or 0

    async def _replace_accounts(self, session: Any, node_id: str, accounts: Any, now: datetime) -> None:
        await session.execute(text("DELETE FROM node_accounts WHERE node_id=:node_id"), {"node_id": node_id})
        for item in accounts if isinstance(accounts, list) else []:
            if not isinstance(item, dict) or not str(item.get("account_id", "")).strip():
                continue
            await session.execute(text("""
                INSERT INTO node_accounts(node_id, account_id, status, credits, ready,
                                          connected, cooldown_until, project_id, metadata, last_seen_at)
                VALUES (:node_id, :account_id, :status, :credits, :ready, :connected,
                        :cooldown, :project_id, CAST(:metadata AS jsonb), :seen)
            """), {
                "node_id": node_id,
                "account_id": str(item["account_id"]),
                "status": str(item.get("status", "active")),
                "credits": item.get("credits"),
                "ready": bool(item.get("ready", False)),
                "connected": bool(item.get("connected", False)),
                "cooldown": item.get("cooldown_until"),
                "project_id": str(item.get("project_id", "")),
                "metadata": _json(item.get("metadata", {})),
                "seen": now,
            })


def _snapshot(row: Any, accounts: list[Any]) -> NodeSnapshot:
    return NodeSnapshot(
        node_id=row["node_id"], advertise_url=row["advertise_url"],
        status=row["status"], max_jobs=row["max_jobs"],
        active_jobs=row["active_jobs"], queued_jobs=row["queued_jobs"],
        capabilities=set((row["capabilities"] or {}).keys()) if isinstance(row["capabilities"], dict) else set(),
        last_heartbeat_at=row["last_heartbeat_at"] or _utcnow(),
        lease_expires_at=row["lease_expires_at"] or datetime.min.replace(tzinfo=timezone.utc),
        accounts={item["account_id"]: dict(item) for item in accounts},
    )


def _json(value: Any) -> str:
    import json
    return json.dumps(value if value is not None else {})
