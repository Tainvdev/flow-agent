from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


@dataclass(slots=True)
class NodeSnapshot:
    """The last heartbeat observed for one external Go service."""

    node_id: str
    advertise_url: str
    status: str = "online"
    max_jobs: int = 1
    active_jobs: int = 0
    queued_jobs: int = 0
    accounts: dict[str, dict[str, Any]] = field(default_factory=dict)
    capabilities: set[str] = field(default_factory=set)
    last_heartbeat_at: datetime = field(default_factory=utcnow)
    lease_expires_at: datetime = field(default_factory=utcnow)

    @property
    def healthy(self) -> bool:
        return self.status == "online" and self.lease_expires_at > utcnow()

    def owns_account(self, account_id: str) -> bool:
        account = self.accounts.get(account_id)
        return bool(account and account.get("ready", True))

    def score(self) -> float:
        capacity = max(1, self.max_jobs)
        return (self.active_jobs + self.queued_jobs) / capacity


@dataclass(slots=True)
class JobRoute:
    gateway_job_id: str
    node_id: str
    account_id: str = ""
    node_job_id: str = ""
    status: str = "routing"


@dataclass(slots=True)
class RoutedJob:
    job_id: str
    kind: str
    payload: dict[str, Any]
    idempotency_key: str
    route: JobRoute | None = None
    status: str = "routing"
    result: dict[str, Any] | None = None
    error: str = ""
