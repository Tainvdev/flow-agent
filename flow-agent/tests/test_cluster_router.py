from datetime import datetime, timedelta, timezone

import pytest

from flow_agent.cluster.models import NodeSnapshot
from flow_agent.cluster.router import RoutingError, StickyRouter


def node(node_id: str, *, account: str = "", active: int = 0, max_jobs: int = 4) -> NodeSnapshot:
    now = datetime.now(timezone.utc)
    accounts = {account: {"account_id": account, "ready": True}} if account else {}
    return NodeSnapshot(
        node_id=node_id, advertise_url=f"http://{node_id}", max_jobs=max_jobs,
        active_jobs=active, accounts=accounts,
        last_heartbeat_at=now, lease_expires_at=now + timedelta(minutes=1),
    )


def test_explicit_account_is_sticky_to_its_owner():
    router = StickyRouter()
    selected = router.choose([node("b", account="acct-b"), node("a", account="acct-a")], kind="image", account_id="acct-a")
    assert selected.node_id == "a"


def test_auto_route_uses_lowest_load_then_node_id():
    router = StickyRouter()
    selected = router.choose([node("b", active=1), node("a", active=1)], kind="video")
    assert selected.node_id == "a"


def test_sticky_node_must_remain_healthy():
    router = StickyRouter()
    with pytest.raises(RoutingError):
        router.choose([node("a", active=4)], kind="image", existing_node_id="a")


def test_unknown_account_is_not_routed_to_another_node():
    router = StickyRouter()
    with pytest.raises(RoutingError):
        router.choose([node("a", account="acct-a")], kind="image", account_id="acct-missing")
