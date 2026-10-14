# Flow Agent — REST API Reference

Complete HTTP API documentation for the **Flow Agent** server.  
All endpoints assume the server is running at its default address.

- **Default Base URL**: `http://127.0.0.1:8001`
- **Swagger UI** (interactive): `http://127.0.0.1:8001/docs`
- **ReDoc**: `http://127.0.0.1:8001/redoc`

---

## Table of Contents

1. [Endpoints Overview](#1-endpoints-overview)
2. [Native Flow Endpoints (`/api/v1/*`)](#2-native-flow-endpoints)
3. [OpenAI-Compatible Endpoints (`/v1/*`)](#3-openai-compatible-endpoints)
4. [System Endpoints](#4-system-endpoints)
5. [Route Aliases](#5-route-aliases)
6. [Error Responses](#6-error-responses)
7. [Client Examples](#7-client-examples)

---

## 1. Endpoints Overview

### Native Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/v1/image` | Generate image(s) from a text prompt |
| `POST` | `/api/v1/video` | Generate a video clip (text or image-to-video) |
| `GET` | `/api/v1/balance` | View credit balance across all accounts |
| `GET` | `/api/v1/stats` | View generation history and cumulative metrics |
| `GET` | `/api/v1/projects` | List Google Flow projects for an account |
| `GET` | `/api/v1/media/{filename}` | Download a generated image or video file |

### OpenAI-Compatible Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/v1/images/generations` | DALL-E–style image generation |
| `POST` | `/v1/chat/completions` | Chat completion with image output + SSE support |

### System Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/health` | Engine liveness check |

> **Route Aliases**: Every `/api/v1/*` native endpoint is also reachable under `/v1/*` for convenience. See [Section 5](#5-route-aliases) for the full alias map.

---

## 2. Native Flow Endpoints

### 2.1 Generate Image

```
POST /api/v1/image
```

Generates one or more images from a text prompt using Google Flow's image models.

#### Request Body

```json
{
  "prompt": "a majestic cybernetic lion in neon rain, 8k cinematic",
  "aspect": "16:9",
  "count": 1,
  "model": "narwhal",
  "all_accounts": false,
  "cookies": null
}
```

| Field | Type | Required | Default | Description |
| :--- | :--- | :---: | :--- | :--- |
| `prompt` | string | ✅ | — | Scene description |
| `aspect` | string | ❌ | `"1:1"` | Aspect ratio — see values below |
| `count` | integer | ❌ | `1` | Number of variations — `1` to `4` |
| `model` | string | ❌ | `"narwhal"` | Model key — `"narwhal"`, `"harbor_seal"`, `"gem_pix_2"` |
| `all_accounts` | boolean | ❌ | `false` | Snapshot eligible accounts at acceptance, create one durable child job per account, then return per-account results and partial failures |
| `account_id` | string | Optional | `null` | Run on one registered account; preferred selector in service mode. |
| `cookies` | string | ❌ | `null` | Path to a specific account bundle, e.g. `"cookies/account_abc.json"` |

**`aspect` valid values:**

| Value | Description |
| :--- | :--- |
| `"1:1"` / `"square"` | Square |
| `"16:9"` / `"landscape"` | Widescreen landscape |
| `"9:16"` / `"portrait"` | Vertical portrait |
| `"4:3"` | Classic landscape |
| `"3:4"` | Classic portrait |

#### Response `200 OK`

```json
{
  "job_id": "ba884266-3fe5-4106-89a1-2ed79c8a8e5b",
  "account_id": "acct-e50d5ecec52e",
  "project_id": "07543949-1b85-4161-85a7-09387d3a95fc",
  "model": "NARWHAL",
  "status": "succeeded",
  "elapsed_seconds": 26.3,
  "files": [
    {
      "media_id": "38e60406-67b3-...",
      "path": "/Users/.../flow-agent/output/acct-38e60406-67b.jpg",
      "url": "http://127.0.0.1:8001/api/v1/media/acct-38e60406-67b.jpg"
    }
  ]
}
```

#### Error Responses

| Status | Condition |
| :--- | :--- |
| `422 Unprocessable Entity` | `count` out of range `1–4`, missing `prompt`, etc. |
| `400 Bad Request` | Unsupported `size` value (OpenAI compat path) |
| `502 Bad Gateway` | Google Flow refused generation (anti-abuse / rate limit) |

---

### 2.2 Generate Video

```
POST /api/v1/video
```

Generates a video clip from text, or animates a starting image into video.

#### Request Body

```json
{
  "prompt": "hyper-lapse of clouds over neon skyscraper city",
  "aspect": "16:9",
  "duration": "8s",
  "quality": "720p",
  "count": 1,
  "start_image": null,
  "all_accounts": false,
  "cookies": null
}
```

| Field | Type | Required | Default | Description |
| :--- | :--- | :---: | :--- | :--- |
| `prompt` | string | ✅ | — | Video description |
| `aspect` | string | ❌ | `"landscape"` | `"landscape"`, `"portrait"`, `"16:9"`, `"9:16"` |
| `duration` | string | ❌ | `"8s"` | Clip length: `"4s"`, `"6s"`, `"8s"`, `"10s"` |
| `quality` | string | ❌ | `"720p"` | Resolution: `"360p"`, `"720p"` |
| `count` | integer | ❌ | `1` | Number of variations — `1` to `4` |
| `start_image` | string | ❌ | `null` | Local image path or uploaded media ID for image-to-video |
| `all_accounts` | boolean | ❌ | `false` | Snapshot eligible accounts at acceptance and fan out through durable child jobs |
| `account_id` | string | Optional | `null` | Run on one registered account; preferred selector in service mode. |
| `cookies` | string | ❌ | `null` | Path to a specific account bundle |

#### Response `200 OK`

```json
{
  "job_id": "5b3f0217-9df1-4658-b806-fe26832a9bb2",
  "account_id": "acct-e50d5ecec52e",
  "status": "succeeded",
  "elapsed_seconds": 58.4,
  "files": [
    {
      "media_id": "4cb43f55-2897-...",
      "path": "/Users/.../flow-agent/output/acct-video-1.mp4",
      "url": "http://127.0.0.1:8001/api/v1/media/acct-video-1.mp4"
    }
  ]
}
```

---

### 2.3 Check Balance

```
GET /api/v1/balance
GET /api/v1/balance?refresh=true
```

Returns available generation credits across all configured accounts.

#### Query Parameters

| Param | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `refresh` | boolean | `false` | `false` = instant read from SQLite cache; `true` = live probe to Google Flow (adds ~5–8 s) |

#### Response `200 OK`

```json
{
  "accounts": [
    {
      "account_id": "acct-1a102fc1d7a1",
      "credits": 50,
      "status": "active"
    },
    {
      "account_id": "acct-e50d5ecec52e",
      "credits": 1049,
      "status": "active"
    }
  ],
  "total_credits": 1099,
  "unread": 0
}
```

---

### 2.4 Generation Statistics

```
GET /api/v1/stats
```

Returns the full generation history and cumulative metrics stored in `data/flow.db`.

#### Response `200 OK`

```json
{
  "generations": [
    {
      "job_id": "ba884266-3fe5-4106-89a1-2ed79c8a8e5b",
      "account_id": "acct-e50d5ecec52e",
      "kind": "image",
      "model": "NARWHAL",
      "aspect": "16:9",
      "status": "succeeded",
      "credits_spent": null,
      "elapsed_seconds": 26.3,
      "created_at": "2026-09-25 10:37:02"
    }
  ],
  "totals": {
    "generations": 1,
    "by_status": { "succeeded": 1 },
    "by_kind": { "image": 1 },
    "media_files": 1,
    "credits_spent": 0
  },
  "latency": {
    "prepare_to_submit": {"samples": 42, "p50_ms": 820, "p95_ms": 1910},
    "submit_to_media": {"samples": 39, "p50_ms": 23600, "p95_ms": 48100},
    "render_wait": {"samples": 18, "p50_ms": 16400, "p95_ms": 39200},
    "download": {"samples": 18, "p50_ms": 740, "p95_ms": 1800}
  },
  "latency_by_kind": {
    "image": {
      "submit_to_media": {"samples": 21, "p50_ms": 1100, "p95_ms": 2450}
    },
    "video": {
      "submit_to_media": {"samples": 18, "p50_ms": 23600, "p95_ms": 48100}
    }
  },
  "service": {
    "queue": {
      "queued": 2,
      "running": 3,
      "waiting_children": 0,
      "outstanding": 5,
      "capacity": 1000,
      "submission_unknown": 1,
      "oldest_queued_age_seconds": 4.2
    },
    "ready_accounts": 5,
    "connected_accounts": 4,
    "strict_cookie_freshness": true,
    "cookie_sync_age_seconds": {"acct-e50d5ecec52e": 1.8, "acct-a818c2ffcf31": 49.1}
  }
}
```

> `credits_spent` is `null` when Google Flow does not return a credit deduction value for that job.
> `latency` reports p50/p95 milliseconds from the latest 5,000 attempts with complete phase pairs; absent stages are omitted until samples exist.
> `latency_by_kind` reports the same bounded phase summaries grouped by job kind (`image`, `video`, or upload kinds) to compare queue wait, preparation, upstream acceptance, render, download and request completion.
> In Go service mode, `service.queue` reports durable job state counts and the age of the oldest queued job; account counts and cookie bundle ages are point-in-time readiness signals. Missing bundle ages are omitted.

---

### 2.5 List Projects

```
GET /api/v1/projects?account_id=acct-e50d5ecec52e
```

Lists Google Flow projects for the selected registered account. Omit `account_id` to let the service select an eligible account. `cookies` path selection at the FastAPI compatibility boundary is translated to the registered account ID before calling the Go service.

#### Response `200 OK`

```json
[
  {
    "project_id": "07543949-1b85-4161-85a7-09387d3a95fc",
    "display_name": "My Flow Project",
    "thumbnail": "https://...",
    "last_asset_id": "f8622c5f-be69-..."
  }
]
```

---

### 2.6 Serve Media Asset

```
GET /api/v1/media/{filename}
GET /v1/media/{filename}
```

Streams a generated image or video file from the `output/` directory.

#### Path Parameters

| Param | Description |
| :--- | :--- |
| `filename` | Filename inside `output/`, e.g. `acct-38e60406-67b.jpg` |

#### Notes

- **MIME Types**: `.jpg`/`.jpeg` → `image/jpeg`, `.png` → `image/png`, `.mp4` → `video/mp4`, `.webp` → `image/webp`.
- **Security**: `{filename}` is validated to resolve strictly inside `output/`. Path traversal attempts (e.g. `../../etc/passwd`) return **`404 Not Found`**.

#### Error Responses

| Status | Condition |
| :--- | :--- |
| `404 Not Found` | File does not exist, or path traversal detected |

---

## 3. OpenAI-Compatible Endpoints

These endpoints conform to the OpenAI HTTP API contract and work with standard OpenAI client libraries.

### 3.1 Images Generations

```
POST /v1/images/generations
```

Drop-in replacement for `openai.images.generate(...)`. Accepts standard OpenAI request shapes and returns a standard response.

#### Request Body

```json
{
  "prompt": "futuristic city in autumn, digital art",
  "n": 1,
  "size": "1024x1024",
  "response_format": "url"
}
```

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `prompt` | string | — | Image description (required) |
| `n` | integer | `1` | Number of images — `1` to `4` |
| `size` | string | `"1024x1024"` | See size mapping below |
| `response_format` | string | `"url"` | `"url"` or `"b64_json"` |

**`size` → Flow aspect mapping:**

| `size` value | Flow aspect |
| :--- | :--- |
| `"1024x1024"` / `"square"` | `1:1` |
| `"1792x1024"` / `"landscape"` | `16:9` |
| `"1024x1792"` / `"portrait"` | `9:16` |
| `"1365x1024"` | `4:3` |
| `"1024x1365"` | `3:4` |

#### Response `200 OK`

```json
{
  "created": 1727201000,
  "data": [
    {
      "url": "http://127.0.0.1:8001/api/v1/media/acct-38e60406-67b.jpg"
    }
  ],
  "flow": {
    "job_id": "ba884266-3fe5-4106-89a1-2ed79c8a8e5b",
    "account_id": "acct-e50d5ecec52e",
    "status": "succeeded"
  }
}
```

When `response_format` is `"b64_json"`, each item contains `b64_json` instead of `url`.

#### Error Responses

| Status | Condition |
| :--- | :--- |
| `400 Bad Request` | Unsupported `size` value |
| `422 Unprocessable Entity` | `n` > 4 or missing `prompt` |
| `502 Bad Gateway` | Google Flow refused generation |

---

### 3.2 Chat Completions

```
POST /v1/chat/completions
```

Generates an image from conversation history and returns a markdown-formatted response. Supports real-time streaming via SSE.

#### Request Body

```json
{
  "model": "flow",
  "messages": [
    { "role": "system", "content": "You are an AI illustrator." },
    { "role": "user", "content": "Draw an origami hummingbird near a blossom" }
  ],
  "stream": false
}
```

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `model` | string | `"flow"` | Any value accepted (ignored, always uses Flow) |
| `messages` | array | — | Conversation array — last `user` message is used as the prompt |
| `stream` | boolean | `false` | `true` enables Server-Sent Events streaming |

#### Response `200 OK` (non-streaming)

```json
{
  "id": "chatcmpl-flow-ba884266",
  "object": "chat.completion",
  "created": 1727201000,
  "model": "flow",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "![Generated Image](http://127.0.0.1:8001/api/v1/media/acct-38e60406-67b.jpg)"
      },
      "finish_reason": "stop"
    }
  ]
}
```

When `stream: true`, the response is an SSE stream of `data: {...}` chunks followed by `data: [DONE]`.

---

## 4. System Endpoints

### 4.1 Health Check

```
GET /health
```

Confirms the Python server is running and its configured engine backend is reachable.

#### Response `200 OK`

```json
{
  "status": "ok",
  "engine": true,
  "mode": "service",
  "binary": "/Users/.../flow-agent/bin/flow-macos"
}
```

| Field | Description |
| :--- | :--- |
| `status` | Always `"ok"` if server is up |
| `engine` | `true` if the Go service answers health checks in service mode, or the binary exists in subprocess mode |
| `ready_accounts` | Warm account runtimes available in service mode |
| `connected_accounts` | Accounts with a live extension peer; strict cookie sync needs these |
| `strict_cookie_freshness` | Whether service jobs require live extension sync before submit |
| `mode` | `service` or `subprocess` |
| `binary` | Absolute path selected for subprocess mode; informational in service mode |

---

## 5. Route Aliases

Every native `/api/v1/*` endpoint is also reachable under a `/v1/*` alias. This allows a single `base_url` of `http://127.0.0.1:8001/v1` for all endpoints.

| Canonical Route | Alias(es) |
| :--- | :--- |
| `POST /api/v1/image` | `POST /v1/image`, `POST /v1/generate/image` |
| `POST /api/v1/video` | `POST /v1/video`, `POST /v1/generate/video` |
| `GET /api/v1/balance` | `GET /v1/balance` |
| `GET /api/v1/stats` | `GET /v1/stats` |
| `GET /api/v1/projects` | `GET /v1/projects` |
| `GET /api/v1/media/{filename}` | `GET /v1/media/{filename}` |

The OpenAI routes (`/v1/images/generations`, `/v1/chat/completions`) are **only** available under `/v1/` — they have no `/api/v1/` equivalent.

---

## 6. Error Responses

All errors return a JSON body with a `detail` field:

```json
{
  "detail": "count must be between 1 and 4"
}
```

| HTTP Status | Meaning |
| :--- | :--- |
| `400 Bad Request` | Invalid parameter value (e.g. unsupported image size) |
| `404 Not Found` | Media file not found or path traversal blocked |
| `422 Unprocessable Entity` | Request body validation failure (missing required field, out-of-range value) |
| `502 Bad Gateway` | Google Flow upstream refused the generation request (anti-abuse / rate limit) — not a server error |

> **Note on 502:** A `502` means the code path executed correctly and Google declined the request. Retrying after a cooldown of 30–120 s usually succeeds. Attaching the Flow Chrome extension and using `--captcha broker` significantly reduces the chance of refusal.

---

## 7. Client Examples

### Native asynchronous jobs (service mode)

The compatibility generation endpoints above continue waiting for their final result. For callers that want an early acknowledgement, service mode exposes:

- `POST /api/v1/jobs` with `{ "kind": "image|video|upload-video", "payload": { ... } }`; returns HTTP 202 and a `job_id`/`status_url`.
- `GET /api/v1/jobs/{job_id}` to read durable status and result.
- `GET /api/v1/jobs/{job_id}/events` streams status and attempt-phase events as SSE. Event IDs are monotonic; send `Last-Event-ID` to resume after reconnecting. The stream ends after the terminal status event.
- Job status includes `attempt_history`; each attempt now carries ordered `phase_events` with timestamps so callers can measure preparation, submission, rendering, and download intervals across restarts.
- `POST /api/v1/jobs/{job_id}/cancel` to request cancellation. Disconnecting from the POST does not cancel accepted work.
- If Go committed a job but its POST acknowledgement was lost, the Python adapter recovers it with the same idempotency key. If it cannot resolve the outcome, the response carries `Idempotency-Key` so the caller can safely retry with the same key.
- If a compatibility generation wait reaches its deadline after acceptance, the response includes `X-Flow-Job-ID`; use that ID with `GET /api/v1/jobs/{job_id}` to continue tracking the durable job.
- Fan-out parents may initially report `waiting_children`; each account/count child is independently queued, and the parent becomes `succeeded`, `partial`, or `failed` after all children are terminal.
- Image `count` children share one account selected when the parent is accepted, and their generated media/files are combined into the parent result.
- Service mode validates generation payloads locally before accepting a durable job. Missing prompts/upload paths and invalid image count, aspect, or video duration return HTTP 422 without consuming queue capacity.

Send an `Idempotency-Key` header when retrying an uncertain client request. The compatibility generation routes forward that header too. These routes require `FLOW_ENGINE_MODE=service`, `FLOW_SERVICE_URL` and `FLOW_INTERNAL_TOKEN` on the FastAPI process. Set the same absolute `FLOW_OUTPUT_DIR` in the Go service and FastAPI process so generated files are served from the shared output directory.

### Python — OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8001/v1",
    api_key="not-needed"
)

# Image generation
response = client.images.generate(
    prompt="a glowing crystal cavern, unreal engine 5",
    size="1792x1024",   # → Flow 16:9
    n=1
)
print("Image URL:", response.data[0].url)

# Chat with image output
response = client.chat.completions.create(
    model="flow",
    messages=[{"role": "user", "content": "paint a misty forest at dawn"}]
)
print(response.choices[0].message.content)
```

### Python — httpx (Native API)

```python
import httpx

BASE = "http://127.0.0.1:8001"

# Generate image
r = httpx.post(f"{BASE}/api/v1/image", json={
    "prompt": "emerald dragon over misty mountains",
    "aspect": "16:9",
    "count": 1
}, timeout=120)
result = r.json()
print("Job:", result["job_id"])
print("File:", result["files"][0]["url"])

# Check balance
r = httpx.get(f"{BASE}/api/v1/balance")
print("Total credits:", r.json()["total_credits"])
```

### cURL

```bash
# Generate an image
curl -X POST http://127.0.0.1:8001/api/v1/image \
  -H "Content-Type: application/json" \
  -d '{"prompt": "emerald dragon over misty mountains", "aspect": "16:9"}'

# Check balance (live refresh)
curl "http://127.0.0.1:8001/api/v1/balance?refresh=true"

# Download a generated file
curl -O http://127.0.0.1:8001/api/v1/media/acct-38e60406-67b.jpg

# Health check
curl http://127.0.0.1:8001/health
```

### Using `/v1` aliases with any OpenAI-compatible tool

```bash
# Works with LangChain, LiteLLM, Cursor, etc.
export OPENAI_BASE_URL="http://127.0.0.1:8001/v1"
export OPENAI_API_KEY="not-needed"
```
