from __future__ import annotations

from dataclasses import dataclass
from typing import Iterable

from .models import NodeSnapshot


class RoutingError(RuntimeError):
    """A job cannot be safely assigned to a healthy node."""


@dataclass(slots=True)
class StickyRouter:
    """Deterministic routing policy.

    Persistence is deliberately outside this class. The database transaction
    must reserve the selected node before the caller performs network I/O.
    """

    def choose(
        self,
        nodes: Iterable[NodeSnapshot],
        *,
        kind: str,
        account_id: str = "",
        existing_node_id: str = "",
        excluded_node_ids: set[str] | None = None,
    ) -> NodeSnapshot:
        available = [
            node for node in nodes
            if node.healthy
            and node.node_id not in (excluded_node_ids or set())
            and (not node.capabilities or kind in node.capabilities)
            and node.active_jobs < max(1, node.max_jobs)
        ]
        if existing_node_id:
            for node in available:
                if node.node_id == existing_node_id:
                    if account_id and not node.owns_account(account_id):
                        raise RoutingError(
                            f"node {existing_node_id!r} does not own account {account_id!r}"
                        )
                    return node
            raise RoutingError(f"sticky node {existing_node_id!r} is unavailable")

        if account_id:
            available = [node for node in available if node.owns_account(account_id)]
            if not available:
                raise RoutingError(f"no healthy node owns account {account_id!r}")

        if not available:
            raise RoutingError(f"no healthy node can accept kind {kind!r}")

        return min(available, key=lambda node: (node.score(), node.node_id))
