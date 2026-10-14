import unittest
from unittest.mock import patch

from scripts import service_cutover_preflight as preflight


class ServiceCutoverPreflightTests(unittest.TestCase):
    def setUp(self):
        self.responses = {
            "http://go/health": (200, {
                "status": "ok", "ready_accounts": 2, "connected_accounts": 2,
                "strict_cookie_freshness": True,
            }),
            "http://go/readyz": (200, {"status": "ready"}),
            "http://go/internal/v1/stats": (200, {"service": {"queue": {}}}),
            "http://api/health": (200, {
                "status": "ok", "mode": "service", "engine": True,
                "ready_accounts": 2, "connected_accounts": 2,
                "strict_cookie_freshness": True,
            }),
        }

    def verify(self, **overrides):
        values = {
            "mode": "service", "service_url": "http://go/", "token": "secret",
            "api_url": "http://api", "cookie_freshness": "strict",
        }
        values.update(overrides)

        def get_json(url, token="", timeout=3.0):
            if url.endswith("/internal/v1/stats"):
                self.assertEqual(token, "secret")
            else:
                self.assertEqual(token, "")
            return self.responses[url]

        with patch.object(preflight, "get_json", side_effect=get_json):
            return preflight.verify_cutover(**values)

    def test_ready_service_and_fastapi_pass(self):
        self.assertEqual(self.verify(), [])

    def test_requires_explicit_service_configuration(self):
        failures = self.verify(mode="", service_url="", token="")
        self.assertEqual(len(failures), 3)

    def test_rejects_unavailable_go_readiness(self):
        self.responses["http://go/readyz"] = (503, {"status": "not_ready"})
        failures = self.verify()
        self.assertIn("Go service /readyz is not ready", failures)

    def test_strict_cookie_mode_requires_connected_account(self):
        self.responses["http://go/health"][1]["connected_accounts"] = 0
        failures = self.verify()
        self.assertIn("strict cookie freshness is enabled but no extension account is connected", failures)

    def test_cached_cookie_mode_must_be_explicit(self):
        self.responses["http://go/health"][1]["strict_cookie_freshness"] = False
        self.responses["http://api/health"][1]["strict_cookie_freshness"] = False
        failures = self.verify(cookie_freshness="")
        self.assertIn("cached-cookie mode must be explicitly configured when strict freshness is off", failures)
        self.assertEqual(self.verify(cookie_freshness="cached"), [])

    def test_requires_stats_schema_and_reported_freshness_policy(self):
        self.responses["http://go/internal/v1/stats"] = (200, {})
        self.responses["http://go/health"][1].pop("strict_cookie_freshness")
        failures = self.verify()
        self.assertIn("Go service rejected the bearer token or stats request", failures)
        self.assertIn("Go service did not report its cookie freshness policy", failures)

    def test_fastapi_must_use_service_mode_and_match_account_health(self):
        self.responses["http://api/health"][1]["mode"] = "subprocess"
        failures = self.verify()
        self.assertIn("FastAPI is not connected in service mode", failures)


if __name__ == "__main__":
    unittest.main()
