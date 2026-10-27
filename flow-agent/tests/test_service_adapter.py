import json
import base64
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import AsyncMock, patch

import httpx

# Keep the import-time FastAPI/MCP singleton explicit in unittest runs too;
# individual mode validation tests clear the environment as needed.
os.environ.setdefault("FLOW_ENGINE_MODE", "subprocess")

from flow_agent.engine import EngineError, FlowEngine
from flow_agent import mcp_server
from flow_agent import api as flow_api
from flow_agent.api import _engine_http_status


class EngineModeTests(unittest.TestCase):
    def test_service_environment_requires_an_explicit_mode(self):
        with patch.dict(
            "os.environ",
            {"FLOW_SERVICE_URL": "http://flow.local", "FLOW_INTERNAL_TOKEN": "secret"},
            clear=True,
        ):
            with self.assertRaisesRegex(ValueError, "FLOW_ENGINE_MODE=service"):
                FlowEngine()

    def test_explicit_service_mode_requires_service_url(self):
        with patch.dict("os.environ", {"FLOW_ENGINE_MODE": "service"}, clear=True):
            with self.assertRaisesRegex(ValueError, "FLOW_SERVICE_URL is required"):
                FlowEngine()

    def test_explicit_service_mode_requires_internal_token(self):
        with patch.dict(
            "os.environ",
            {"FLOW_ENGINE_MODE": "service", "FLOW_SERVICE_URL": "http://flow.local"},
            clear=True,
        ):
            with self.assertRaisesRegex(ValueError, "FLOW_INTERNAL_TOKEN is required"):
                FlowEngine()

    def test_explicit_subprocess_mode_rejects_service_url_conflict(self):
        with patch.dict(
            "os.environ",
            {"FLOW_ENGINE_MODE": "subprocess", "FLOW_SERVICE_URL": "http://flow.local"},
            clear=True,
        ):
            with self.assertRaisesRegex(ValueError, "FLOW_SERVICE_URL is set"):
                FlowEngine()

    def test_no_engine_mode_fails_closed(self):
        with patch.dict("os.environ", {}, clear=True):
            with self.assertRaisesRegex(ValueError, "FLOW_ENGINE_MODE must be explicitly set"):
                FlowEngine()

    def test_programmatic_service_url_is_an_explicit_service_choice(self):
        with patch.dict("os.environ", {}, clear=True):
            self.assertEqual(
                FlowEngine(service_url="http://flow.local", service_token="token").engine_mode,
                "service",
            )


class ServiceAdapterTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.engine = FlowEngine(
            service_url="http://flow.local",
            service_token="test-token",
            timeout=3,
            mode="service",
        )

    async def asyncTearDown(self):
        await self.engine.aclose()

    async def test_submit_once_then_poll_until_succeeded(self):
        requests = []
        states = iter(
            [
                {"status": "queued"},
                {"status": "succeeded", "result": {"job_id": "job-1", "media": ["m1"]}},
            ]
        )

        def handler(request):
            requests.append(request)
            self.assertEqual(request.headers["Authorization"], "Bearer test-token")
            self.assertEqual(request.headers["Idempotency-Key"], "client-key")
            if request.method == "POST":
                self.assertEqual(request.url.path, "/internal/v1/jobs")
                self.assertEqual(
                    json.loads(request.content),
                    {"kind": "video", "payload": {"prompt": "test"}, "payload_version": 1},
                )
                return httpx.Response(202, json={"job_id": "job-1", "status": "queued"})
            self.assertEqual(request.url.path, "/internal/v1/jobs/job-1")
            return httpx.Response(200, json=next(states))

        self.engine._service_http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        result = await self.engine._run_service_job(
            "video", {"prompt": "test"}, idempotency_key="client-key"
        )

        self.assertEqual(result, {"job_id": "job-1", "media": ["m1"]})
        self.assertEqual([request.method for request in requests], ["POST", "GET", "GET"])

    async def test_job_events_resume_and_status_link_is_public(self):
        requests = []
        stream_body = b'id: 8\nevent: status\ndata: {"status":"succeeded"}\n\n'

        def handler(request):
            requests.append(request)
            if request.url.path.endswith("/events"):
                return httpx.Response(
                    200,
                    headers={"Content-Type": "text/event-stream"},
                    content=stream_body,
                )
            return httpx.Response(
                200,
                json={"job_id": "job-events", "status": "succeeded", "events_url": "/internal/v1/jobs/job-events/events"},
            )

        self.engine._service_http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        job = await self.engine.get_service_job("job-events")
        self.assertEqual(job["events_url"], "/api/v1/jobs/job-events/events")
        response = await self.engine.open_service_job_events("job-events", "7")
        self.assertEqual(response.status_code, 200)
        self.assertEqual(await response.aread(), stream_body)
        await response.aclose()
        self.assertEqual(requests[-1].url.path, "/internal/v1/jobs/job-events/events")
        self.assertEqual(requests[-1].headers["Authorization"], "Bearer test-token")
        self.assertEqual(requests[-1].headers["Last-Event-ID"], "7")

    async def test_terminal_account_unavailable_preserves_http_status(self):
        def handler(request):
            if request.method == "POST":
                return httpx.Response(202, json={"job_id": "job-503", "status": "queued"})
            return httpx.Response(
                200,
                json={
                    "job_id": "job-503", "status": "failed",
                    "error": "no account is ready", "error_code": 503,
                },
            )

        self.engine._service_http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        with self.assertRaises(EngineError) as raised:
            await self.engine._run_service_job(
                "image", {"prompt": "test"}, idempotency_key="client-key"
            )
        self.assertEqual(raised.exception.returncode, 503)
        self.assertEqual(raised.exception.job_id, "job-503")
        self.assertEqual(_engine_http_status(raised.exception), 503)

    async def test_api_status_mapping_distinguishes_service_down_from_unknown_submit(self):
        read_failure = EngineError(
            ["http://flow.local", "/internal/v1/balance"], -1, "", "connection refused"
        )
        self.assertEqual(_engine_http_status(read_failure), 503)

        ambiguous_submit = EngineError(
            ["http://flow.local", "image"], -1, "",
            "service submit outcome is unknown; retry with the same key",
            idempotency_key="client-key",
        )
        self.assertEqual(_engine_http_status(ambiguous_submit), 502)

        wait_deadline = EngineError(
            ["http://flow.local", "image"], -1, "", "job is still running",
            job_id="accepted-job",
        )
        self.assertEqual(_engine_http_status(wait_deadline), 504)

    async def test_high_level_generators_use_service_without_subprocess_fallback(self):
        self.engine._run_service_job = AsyncMock(return_value={"status": "succeeded"})
        self.engine._run_json = AsyncMock(side_effect=AssertionError("subprocess path was called"))

        await self.engine.generate_image("image prompt", idempotency_key="image-key")
        await self.engine.generate_video("video prompt", idempotency_key="video-key")
        await self.engine.upload_video("clip.mp4", idempotency_key="upload-key")

        self.assertEqual(self.engine._run_service_job.await_count, 3)
        self.assertEqual(
            [call.args[0] for call in self.engine._run_service_job.await_args_list],
            ["image", "video", "upload-video"],
        )
        self.engine._run_json.assert_not_awaited()

    async def test_service_jobs_send_account_id_not_cookie_file_path(self):
        captured = []

        async def submit(kind, payload, *args, **kwargs):
            captured.append((kind, self.engine._normalize_service_account(payload)))
            return {"status": "succeeded"}

        self.engine._run_service_job = AsyncMock(side_effect=submit)
        await self.engine.generate_image(
            "image prompt", cookies="cookies/account_acct-1.json", idempotency_key="image"
        )
        await self.engine.generate_video(
            "video prompt", account_id="acct-2", idempotency_key="video"
        )
        await self.engine.upload_video(
            "clip.mp4", cookies="/data/cookies/account_acct-3.json", idempotency_key="upload"
        )

        self.assertEqual(captured[0][1]["account_id"], "acct-1")
        self.assertNotIn("cookies", captured[0][1])
        self.assertEqual(captured[1][1]["account_id"], "acct-2")
        self.assertEqual(captured[2][1]["account_id"], "acct-3")
        self.assertTrue(all("cookies" not in payload for _, payload in captured))

    async def test_subprocess_all_accounts_requests_machine_readable_cli_aggregate(self):
        self.engine.engine_mode = "subprocess"
        self.engine._run_json = AsyncMock(return_value={"status": "partial", "accounts": []})

        image = await self.engine.generate_image("draw", all_accounts=True)
        video = await self.engine.generate_video("move", all_accounts=True)

        self.assertEqual(image["status"], "partial")
        self.assertEqual(video["status"], "partial")
        for call in self.engine._run_json.await_args_list:
            self.assertIn("--all", call.args)
            self.assertIn("--json", call.args)

        self.engine._run_json = AsyncMock(return_value={"status": "failed", "accounts": []})
        with self.assertRaises(EngineError) as raised:
            await self.engine.generate_image("draw", all_accounts=True)
        self.assertEqual(raised.exception.returncode, 502)


    async def test_service_account_selector_rejects_mismatch_and_invalid_path(self):
        with self.assertRaises(EngineError) as mismatch:
            await self.engine._run_service_job(
                "image", {"prompt": "x", "account_id": "acct-a", "cookies": "account_acct-b.json"}
            )
        self.assertEqual(mismatch.exception.returncode, 409)
        with self.assertRaises(EngineError) as invalid:
            await self.engine._run_service_job(
                "image", {"prompt": "x", "cookies": "cookies.json"}
            )
        self.assertEqual(invalid.exception.returncode, 422)

    async def test_projects_service_uses_account_id_query(self):
        seen = []

        def handler(request):
            seen.append(request)
            return httpx.Response(200, json=[])

        self.engine._service_http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        await self.engine.get_projects(cookies="cookies/account_acct-1.json")
        self.assertEqual(seen[0].url.params["account_id"], "acct-1")
        self.assertNotIn("cookies", seen[0].url.params)

    async def test_submit_transport_error_is_reported_without_resubmission(self):
        calls = []

        def handler(request):
            calls.append(request)
            raise httpx.ConnectError("connection dropped after submit", request=request)

        self.engine._service_http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        with self.assertRaises(EngineError) as raised:
            await self.engine._run_service_job(
                "image", {"prompt": "test"}, idempotency_key="client-key"
            )

        self.assertIn("outcome is unknown", str(raised.exception))
        self.assertEqual(sum(request.method == "POST" for request in calls), 1)
        self.assertEqual(raised.exception.idempotency_key, "client-key")

    async def test_lost_submit_ack_is_recovered_by_idempotency_key(self):
        calls = {"post": 0, "lookup": 0, "status": 0}

        def handler(request):
            if request.method == "POST":
                calls["post"] += 1
                raise httpx.ConnectError("ack lost after commit", request=request)
            if request.url.path == "/internal/v1/jobs/by-key":
                calls["lookup"] += 1
                self.assertEqual(request.url.params["key"], "client-key")
                return httpx.Response(
                    200, json={"job_id": "job-recovered", "status": "queued"}
                )
            calls["status"] += 1
            return httpx.Response(
                200,
                json={"status": "succeeded", "result": {"job_id": "job-recovered"}},
            )

        self.engine._service_http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        result = await self.engine._run_service_job(
            "image", {"prompt": "test"}, idempotency_key="client-key"
        )
        self.assertEqual(result, {"job_id": "job-recovered"})
        self.assertEqual(calls, {"post": 1, "lookup": 1, "status": 1})

    async def test_async_enqueue_recovers_lost_ack(self):
        def handler(request):
            if request.method == "POST":
                raise httpx.ConnectError("ack lost", request=request)
            return httpx.Response(
                200, json={"job_id": "job-async", "status": "queued"}
            )

        self.engine._service_http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        accepted = await self.engine.enqueue_service_job(
            "video", {"prompt": "test"}, idempotency_key="async-key"
        )
        self.assertEqual(accepted["job_id"], "job-async")

    async def test_partial_result_is_returned_without_waiting_for_success(self):
        def handler(request):
            if request.method == "POST":
                return httpx.Response(202, json={"job_id": "job-2", "status": "queued"})
            return httpx.Response(
                200,
                json={"status": "partial", "result": {"status": "partial", "media": ["m1"]}},
            )

        self.engine._service_http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        result = await self.engine._run_service_job("image", {"prompt": "test"})
        self.assertEqual(result, {"status": "partial", "media": ["m1"]})

    async def test_polling_retries_transient_service_failures_without_resubmitting(self):
        calls = {"post": 0, "get": 0}

        def handler(request):
            if request.method == "POST":
                calls["post"] += 1
                return httpx.Response(202, json={"job_id": "job-3", "status": "queued"})
            calls["get"] += 1
            if calls["get"] == 1:
                raise httpx.ConnectError("temporary disconnect", request=request)
            if calls["get"] == 2:
                return httpx.Response(503, text="temporarily unavailable")
            return httpx.Response(
                200, json={"status": "succeeded", "result": {"job_id": "job-3"}}
            )

        self.engine._service_http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        result = await self.engine._run_service_job("image", {"prompt": "test"})
        self.assertEqual(result, {"job_id": "job-3"})
        self.assertEqual(calls, {"post": 1, "get": 3})

    async def test_wait_deadline_retains_accepted_job_id(self):
        def handler(request):
            return httpx.Response(202, json={"job_id": "job-timeout", "status": "queued"})

        self.engine._service_http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        with self.assertRaises(EngineError) as raised:
            await self.engine._run_service_job("image", {"prompt": "test"}, timeout=0.001)
        self.assertEqual(raised.exception.job_id, "job-timeout")
        self.assertIn("job-timeout", str(raised.exception))

    async def test_unresolved_submit_retains_idempotency_key_for_safe_replay(self):
        self.engine._service_http = httpx.AsyncClient(
            transport=httpx.MockTransport(lambda request: httpx.Response(404))
        )
        with self.assertRaises(EngineError) as raised:
            await self.engine._recover_service_submission(
                self.engine._service_http, "client-key", ["service", "image"], 0
            )
        self.assertEqual(raised.exception.idempotency_key, "client-key")

    async def test_mcp_tools_use_service_for_generation_upload_and_reads(self):
        requests = []

        def handler(request):
            requests.append(request)
            self.assertEqual(request.headers["Authorization"], "Bearer test-token")
            if request.method == "POST":
                body = json.loads(request.content)
                job_id = f"{body['kind']}-job"
                return httpx.Response(202, json={"job_id": job_id, "status": "queued"})
            if request.url.path.startswith("/internal/v1/jobs/"):
                job_id = request.url.path.rsplit("/", 1)[-1]
                return httpx.Response(
                    200,
                    json={
                        "status": "succeeded",
                        "result": {"job_id": job_id, "status": "succeeded"},
                    },
                )
            if request.url.path == "/internal/v1/balance":
                return httpx.Response(200, json={"accounts": [], "total_credits": 0})
            if request.url.path == "/internal/v1/stats":
                return httpx.Response(200, json={"totals": {"generations": 0}})
            return httpx.Response(404)

        service_engine = FlowEngine(
            service_url="http://flow.local", service_token="test-token", timeout=3, mode="service"
        )
        service_engine._service_http = httpx.AsyncClient(
            transport=httpx.MockTransport(handler)
        )
        self.addAsyncCleanup(service_engine.aclose)
        with patch.object(mcp_server, "engine", service_engine):
            for name, arguments in (
                ("flow_generate_image", {"prompt": "image"}),
                ("flow_generate_video", {"prompt": "video"}),
                ("flow_upload_video", {"file_path": "C:/tmp/clip.mp4"}),
                ("flow_get_balance", {}),
                ("flow_get_stats", {}),
            ):
                result = await mcp_server.call_tool(name, arguments)
                self.assertNotIn("isError", result, (name, result))

        posts = [request for request in requests if request.method == "POST"]
        self.assertEqual(len(posts), 3)
        self.assertEqual(
            [json.loads(request.content)["kind"] for request in posts],
            ["image", "video", "upload-video"],
        )
        self.assertTrue(any(request.url.path == "/internal/v1/balance" for request in requests))
        self.assertTrue(any(request.url.path == "/internal/v1/stats" for request in requests))


if __name__ == "__main__":
    unittest.main()


class APICompatibilityTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.engine = type("APIEngineStub", (), {})()
        self.engine.generate_image = AsyncMock()
        self.engine.generate_video = AsyncMock()
        self.engine.enqueue_service_job = AsyncMock()
        self.engine.get_service_job = AsyncMock()
        self.engine.aclose = AsyncMock()
        self.engine.generate_image.return_value = {
            "job_id": "compat-image", "status": "succeeded",
            "files": [], "media": ["media-image"],
        }
        self.engine.generate_video.return_value = {
            "job_id": "compat-video", "status": "succeeded",
            "files": ["video.mp4"], "media": ["media-video"],
        }
        self.engine_patch = patch.object(flow_api, "engine", self.engine)
        self.engine_patch.start()
        self.client = httpx.AsyncClient(
            transport=httpx.ASGITransport(app=flow_api.app), base_url="http://api.test"
        )

    async def asyncTearDown(self):
        await self.client.aclose()
        self.engine_patch.stop()

    async def test_openai_image_response_keeps_url_shape_and_flow_metadata(self):
        with tempfile.TemporaryDirectory() as directory:
            image = Path(directory) / "result.png"
            image.write_bytes(b"png-data")
            self.engine.generate_image.return_value = {
                "job_id": "compat-image", "status": "succeeded",
                "files": [{"path": str(image)}], "media": ["media-image"],
            }
            response = await self.client.post(
                "/v1/images/generations",
                json={"prompt": "draw a tree", "size": "1024x1024", "n": 1},
                headers={"Idempotency-Key": "compat-key"},
            )

        self.assertEqual(response.status_code, 200, response.text)
        body = response.json()
        self.assertIsInstance(body["created"], int)
        self.assertEqual(body["data"], [{"url": "http://api.test/api/v1/media/result.png"}])
        self.assertEqual(body["flow"]["job_id"], "compat-image")
        self.engine.generate_image.assert_awaited_once_with(
            prompt="draw a tree", aspect="1:1", count=1, model="narwhal",
            cookies=None,
            idempotency_key="compat-key",
        )

    async def test_openai_image_b64_json_and_chat_completion_shapes(self):
        with tempfile.TemporaryDirectory() as directory:
            image = Path(directory) / "result.png"
            image.write_bytes(b"png-data")
            self.engine.generate_image.return_value = {
                "job_id": "compat-image", "status": "succeeded",
                "files": [{"path": str(image)}], "media": [],
            }
            image_response = await self.client.post(
                "/v1/images/generations",
                json={"prompt": "draw", "response_format": "b64_json"},
            )
        self.assertEqual(image_response.status_code, 200, image_response.text)
        self.assertEqual(image_response.json()["data"], [{"b64_json": base64.b64encode(b"png-data").decode()}])

        self.engine.generate_image.return_value = {
            "job_id": "compat-chat", "status": "succeeded",
            "files": [{"path": "/tmp/chat.png"}], "media": [],
        }
        chat = await self.client.post(
            "/v1/chat/completions",
            json={"model": "flow-test", "messages": [{"role": "user", "content": "make it"}]},
        )
        self.assertEqual(chat.status_code, 200, chat.text)
        body = chat.json()
        self.assertEqual(body["object"], "chat.completion")
        self.assertEqual(body["model"], "flow-test")
        self.assertEqual(body["choices"][0]["message"]["role"], "assistant")
        self.assertIn("http://api.test/api/v1/media/chat.png", body["choices"][0]["message"]["content"])

    async def test_native_image_and_video_routes_keep_engine_result_shapes(self):
        image_result = {"job_id": "native-image", "status": "succeeded", "media": ["image-1"]}
        video_result = {"job_id": "native-video", "status": "succeeded", "media": ["video-1"]}
        self.engine.generate_image.return_value = image_result
        self.engine.generate_video.return_value = video_result

        image = await self.client.post(
            "/api/v1/image",
            json={"prompt": "native image", "aspect": "4:3", "count": 2, "account_id": "acct-a"},
        )
        video = await self.client.post(
            "/api/v1/video",
            json={"prompt": "native video", "aspect": "portrait", "duration": "6s", "quality": "720p", "count": 1},
        )

        self.assertEqual((image.status_code, image.json()), (200, image_result))
        self.assertEqual((video.status_code, video.json()), (200, video_result))
        self.assertEqual(self.engine.generate_image.await_args.kwargs["count"], 2)
        self.assertNotIn("account_id", self.engine.generate_image.await_args.kwargs)
        self.assertEqual(self.engine.generate_video.await_args.kwargs["duration"], "6s")

    async def test_gemini_generate_content_adapts_to_existing_image_engine(self):
        self.engine.generate_image.return_value = {
            "job_id": "gemini-image", "status": "succeeded", "media": ["image-uri"],
        }
        response = await self.client.post(
            "/v1beta/models/nano-banano-2.1:generateContent",
            json={
                "contents": [{"role": "user", "parts": [
                    {"text": "draw a fox"},
                    {"inlineData": {"mimeType": "image/jpeg", "data": "aW1hZ2U="}},
                ]}],
                "generationConfig": {"responseModalities": ["IMAGE"], "aspectRatio": "16:9"},
            },
        )
        self.assertEqual(response.status_code, 200, response.text)
        body = response.json()
        self.assertEqual(body["candidates"][0]["content"]["parts"][0]["fileData"]["fileUri"], "image-uri")
        self.engine.generate_image.assert_awaited_once_with(
            prompt="draw a fox", aspect="16:9", count=1, model="nano_banano_2.1",
            reference_data=[{"base64": "aW1hZ2U=", "mime_type": "image/jpeg", "file_name": "gemini-reference"}],
        )

    async def test_gemini_generate_videos_returns_and_polls_long_running_operation(self):
        self.engine.enqueue_service_job.return_value = {
            "job_id": "gemini-video", "status": "queued",
        }
        response = await self.client.post(
            "/v1beta/models/omni-1.1-flash:predictLongRunning",
            json={
                "instances": [{"prompt": "animate the fox", "referenceImages": [
                    {"image": {"inlineData": {"mimeType": "image/png", "data": "aW1hZ2U="}}, "referenceType": "asset"},
                    {"image": {"inlineData": {"mimeType": "image/jpeg", "data": "aW1hZGU="}}, "referenceType": "asset"},
                ]}],
                "parameters": {"aspectRatio": "16:9", "durationSeconds": "8", "resolution": "360p"},
            },
        )
        self.assertEqual(response.status_code, 200, response.text)
        body = response.json()
        self.assertEqual(body, {
            "name": "operations/flow-gemini-video",
            "done": False,
            "metadata": {"model": "omni-1.1-flash", "jobId": "gemini-video"},
        })
        self.engine.enqueue_service_job.assert_awaited_once_with(
            "video",
            {
                "prompt": "animate the fox", "aspect": "16:9", "duration": "8s",
                "quality": "360p", "count": 1, "model": "omni_1_1_flash",
                "start_image": None,
                "reference_data": [
                    {"base64": "aW1hZ2U=", "mime_type": "image/png", "file_name": "gemini-reference-1"},
                    {"base64": "aW1hZGU=", "mime_type": "image/jpeg", "file_name": "gemini-reference-2"},
                ],
            }, None,
        )
        self.engine.get_service_job.return_value = {
            "job_id": "gemini-video", "status": "succeeded",
            "result": {"job_id": "gemini-video", "status": "succeeded", "media": ["video-uri"]},
        }
        completed = await self.client.get("/v1beta/operations/flow-gemini-video")
        self.assertEqual(completed.status_code, 200, completed.text)
        self.assertTrue(completed.json()["done"])
        self.assertEqual(
            completed.json()["response"]["generateVideoResponse"]["generatedSamples"][0]["video"]["uri"],
            "video-uri",
        )
