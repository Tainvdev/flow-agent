#!/usr/bin/env python3
"""Verify Go service and FastAPI health before enabling production intake."""

from __future__ import annotations

import argparse
import json
import os
import sys
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen
from typing import Any


def get_json(url: str, token: str = "", timeout: float = 3.0) -> tuple[int, dict[str, Any]]:
    headers = {"Accept": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    request = Request(url, headers=headers)
    try:
        response = urlopen(request, timeout=timeout)
    except HTTPError as exc:
        response = exc
    except (URLError, TimeoutError, OSError) as exc:
        raise RuntimeError(f"request failed: {type(exc).__name__}") from None
    with response:
        try:
            body = json.loads(response.read())
        except (json.JSONDecodeError, UnicodeDecodeError):
            body = {}
        status = getattr(response, "status", getattr(response, "code", 0))
        return int(status), body if isinstance(body, dict) else {}


def verify_cutover(
    *,
    mode: str,
    service_url: str,
    token: str,
    api_url: str,
    cookie_freshness: str = "strict",
) -> list[str]:
    failures: list[str] = []
    if mode.strip().lower() != "service":
        failures.append("FLOW_ENGINE_MODE must be service")
    if not service_url.strip():
        failures.append("FLOW_SERVICE_URL is required")
    if not token.strip():
        failures.append("FLOW_INTERNAL_TOKEN is required")
    if failures:
        return failures

    service_url = service_url.strip().rstrip("/")
    api_url = api_url.strip().rstrip("/")
    try:
        health_code, service_health = get_json(f"{service_url}/health")
        ready_code, readiness = get_json(f"{service_url}/readyz")
        stats_code, stats = get_json(f"{service_url}/internal/v1/stats", token)
    except RuntimeError as exc:
        return [f"Go service check failed ({exc})"]

    if health_code != 200 or service_health.get("status") != "ok":
        failures.append("Go service /health is not healthy")
    if ready_code != 200 or readiness.get("status") != "ready":
        failures.append("Go service /readyz is not ready")
    if stats_code != 200 or not isinstance(stats.get("service"), dict):
        failures.append("Go service rejected the bearer token or stats request")

    ready_accounts = service_health.get("ready_accounts", 0)
    connected_accounts = service_health.get("connected_accounts", 0)
    strict_freshness = service_health.get("strict_cookie_freshness")
    if not isinstance(strict_freshness, bool):
        failures.append("Go service did not report its cookie freshness policy")
    if not isinstance(ready_accounts, int) or ready_accounts < 1:
        failures.append("Go service has no ready account runtimes")
    if strict_freshness is True and (not isinstance(connected_accounts, int) or connected_accounts < 1):
        failures.append("strict cookie freshness is enabled but no extension account is connected")
    if strict_freshness is False and cookie_freshness.strip().lower() != "cached":
        failures.append("cached-cookie mode must be explicitly configured when strict freshness is off")

    try:
        api_code, api_health = get_json(f"{api_url}/health")
    except RuntimeError as exc:
        failures.append(f"FastAPI health check failed ({exc})")
        return failures
    if api_code != 200 or api_health.get("status") != "ok":
        failures.append("FastAPI /health is not healthy")
    if api_health.get("mode") != "service" or api_health.get("engine") is not True:
        failures.append("FastAPI is not connected in service mode")
    for field in ("ready_accounts", "connected_accounts", "strict_cookie_freshness"):
        if field in service_health and api_health.get(field) != service_health[field]:
            failures.append(f"FastAPI and Go service disagree on {field}")
    return failures


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--api-url", default=os.getenv("FLOW_API_URL", "http://127.0.0.1:8001"))
    args = parser.parse_args()
    failures = verify_cutover(
        mode=os.getenv("FLOW_ENGINE_MODE", ""),
        service_url=os.getenv("FLOW_SERVICE_URL", ""),
        token=os.getenv("FLOW_INTERNAL_TOKEN", ""),
        api_url=args.api_url,
        cookie_freshness=os.getenv("FLOW_COOKIE_FRESHNESS", "strict"),
    )
    if failures:
        for failure in failures:
            print(f"FAIL {failure}", file=sys.stderr)
        return 1
    print("OK Go service health, readiness, bearer auth, cookie freshness and FastAPI service mode")
    print("Production intake is not changed by this check; perform the approved canary separately.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
