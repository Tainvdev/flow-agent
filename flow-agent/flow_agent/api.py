"""FastAPI application: a native Flow API and an OpenAI-compatible one.

Two surfaces over one engine. The native routes speak the engine's own
vocabulary — aspect ratios, durations, quality keys — and the OpenAI routes
translate the shape an OpenAI client already sends, so an existing SDK or a chat
UI works without knowing anything about Flow.
"""

from __future__ import annotations

from contextlib import asynccontextmanager
import json
import os
import hmac
import re
import shutil
import tempfile
import time
from pathlib import Path
from typing import Any, Literal, Optional

from fastapi import FastAPI, File, Form, Header, HTTPException, Request, UploadFile
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse, HTMLResponse, JSONResponse, StreamingResponse
from pydantic import BaseModel, ConfigDict, Field

from .cluster.errors import EngineError
from .cluster.db import ControlPlane
from .cluster.gateway_engine import GatewayEngine
from .cluster.registry import NodeRegistry

control_plane = ControlPlane()
engine = GatewayEngine(control_plane)
node_registry: NodeRegistry | None = None


def _gateway_token_valid(request: Request) -> bool:
    expected = os.getenv("FLOW_GATEWAY_SECRET", "").strip()
    supplied = request.headers.get("Authorization", "")
    if not expected or not supplied.startswith("Bearer "):
        return False
    return hmac.compare_digest(supplied[7:].strip(), expected)


@asynccontextmanager
async def lifespan(_app: FastAPI):
    global node_registry
    if os.getenv("DATABASE_URL", "").strip():
        await control_plane.initialize()
        node_registry = engine.registry
    try:
        yield
    finally:
        node_registry = None
        await control_plane.close()
        close = getattr(engine, "aclose", None)
        if close is not None:
            await close()


app = FastAPI(
    title="flow-agent",
    version="1.0.0",
    description="Image and video generation on Google Flow.",
    lifespan=lifespan,
)
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=False,
    allow_methods=["*"],
    allow_headers=["*"],
)


def _require_router(request: Request) -> NodeRegistry:
    if not _gateway_token_valid(request):
        raise HTTPException(status_code=401, detail="invalid gateway credential")
    if node_registry is None:
        raise HTTPException(status_code=503, detail="control plane is unavailable")
    return node_registry


@app.post("/internal/router/v1/nodes/register")
async def register_node(request: Request):
    registry = _require_router(request)
    payload = await request.json()
    try:
        node = await registry.register(payload)
    except (TypeError, ValueError) as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    return {
        "node_id": node.node_id,
        "status": node.status,
        "lease_expires_at": node.lease_expires_at,
    }


@app.put("/internal/router/v1/nodes/{node_id}/heartbeat")
async def heartbeat_node(node_id: str, request: Request):
    registry = _require_router(request)
    payload = await request.json()
    try:
        node = await registry.heartbeat(node_id, payload)
    except (TypeError, ValueError) as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    return {
        "node_id": node.node_id,
        "status": node.status,
        "lease_expires_at": node.lease_expires_at,
    }


@app.get("/api/v1/nodes")
async def list_nodes(request: Request):
    registry = _require_router(request)
    await registry.mark_stale()
    nodes = await registry.all()
    return {
        "nodes": [
            {
                "node_id": node.node_id,
                "advertise_url": node.advertise_url,
                "status": node.status if node.healthy else "offline",
                "max_jobs": node.max_jobs,
                "active_jobs": node.active_jobs,
                "queued_jobs": node.queued_jobs,
                "accounts": list(node.accounts.values()),
                "last_heartbeat_at": node.last_heartbeat_at,
                "lease_expires_at": node.lease_expires_at,
            }
            for node in nodes
        ]
    }


@app.patch("/api/v1/nodes/{node_id}")
async def update_node_status(node_id: str, request: Request):
    """Drain or re-enable a node without interrupting its running jobs."""
    registry = _require_router(request)
    payload = await request.json()
    try:
        node = await registry.set_status(node_id, str(payload.get("status", "")).strip())
    except (TypeError, ValueError) as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    if node is None:
        raise HTTPException(status_code=404, detail="node not found")
    return {"node_id": node.node_id, "status": node.status, "healthy": node.healthy}


# OpenAI's image sizes, and the aspect each one means.
#
# The pairs are not invented here: OpenAI's three sizes are 1:1, 3:2 and 2:3, and
# the Flow ratios that match are square, landscape and portrait. The two extra
# entries are Flow's own 4:3 and 3:4, which have no OpenAI size and are reachable
# through the native route.
SIZE_TO_ASPECT = {
    "1024x1024": "1:1",
    "1792x1024": "16:9",
    "1024x1792": "9:16",
    "1365x1024": "4:3",
    "1024x1365": "3:4",
    # The aliases a caller may use instead of pixels.
    "square": "1:1",
    "landscape": "16:9",
    "portrait": "9:16",
}

BASE_DIR = Path(__file__).resolve().parent
MEDIA_DIR = Path(os.getenv("FLOW_OUTPUT_DIR", str(BASE_DIR / "output"))).resolve()
IDENTIFIER = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


# --------------------------------------------------------------------------- #
# request models
# --------------------------------------------------------------------------- #


class ImageGenerationRequest(BaseModel):
    """The OpenAI images payload, plus the two Flow extras.

    Extra fields are allowed rather than rejected: an OpenAI client sends keys
    this does not model, and failing the request over a field nobody needs would
    make the endpoint unusable from the SDKs it exists to serve.
    """

    model_config = ConfigDict(extra="allow")

    prompt: str
    model: str = "narwhal"
    n: int = Field(default=1, ge=1, le=4)
    size: str = "1024x1024"
    response_format: str = "url"
    # Flow-specific, ignored by OpenAI clients.
    cookies: Optional[str] = None


class FlowImageRequest(BaseModel):
    prompt: str
    aspect: str = "1:1"
    count: int = Field(default=1, ge=1, le=4)
    model: str = "narwhal"
    cookies: Optional[str] = None


class FlowVideoRequest(BaseModel):
    prompt: str
    aspect: str = "landscape"
    duration: str = "8s"
    quality: str = "720p"
    count: int = Field(default=1, ge=1, le=4)
    start_image: Optional[str] = None
    cookies: Optional[str] = None


class AsyncJobRequest(BaseModel):
    kind: Literal["image", "video", "upload-video"]
    payload: dict[str, Any]


class RuntimeSettingsRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")
    values: dict[str, Any] = Field(default_factory=dict)
    reset: list[str] = Field(default_factory=list)


class ChatMessage(BaseModel):
    model_config = ConfigDict(extra="allow")

    role: str
    content: Any = ""


class ChatCompletionRequest(BaseModel):
    model_config = ConfigDict(extra="allow")

    model: str = "flow"
    messages: list[ChatMessage]
    stream: bool = False
    # Flow-specific.
    size: str = "1024x1024"
    cookies: Optional[str] = None


class GeminiVideoRequest(BaseModel):
    """Small Gemini Veo-compatible request subset.

    The Python adapter translates this shape to the existing Flow video
    request. The upstream Go transport remains unchanged.
    """

    model_config = ConfigDict(extra="allow")

    instances: list[dict[str, Any]] = Field(default_factory=list)
    parameters: dict[str, Any] = Field(default_factory=dict)


GEMINI_IMAGE_MODELS = {
    "nano-banano-2.1": "nano_banano_2.1",
    "nano-banano-2-lite": "nano_banana_2_lite",
    "nano-banano-pro": "nano_banana_pro",
}

GEMINI_VIDEO_MODELS = {
    "omni-1.1-flash": "omni_1_1_flash",
    "omni_1_1_flash": "omni_1_1_flash",
    "veo-3.1-lite": "360p",
    "veo-3.1-fast": "720p",
    "veo-3.1-quality": "720p",
}


# --------------------------------------------------------------------------- #
# helpers
# --------------------------------------------------------------------------- #


def _no_image_detail(result: dict) -> str:
    """Why a run that exited zero produced nothing usable.

    The engine reports a refusal in the result rather than by failing, so this
    is where the two are told apart. The status and job id are named because
    they are what a caller needs to look the run up; `status` is `empty` for a
    refusal and `succeeded` for a job whose files could not be read.
    """
    return (
        "the engine produced no image "
        f"(status={result.get('status')!r}, job={result.get('job_id')!r}, "
        f"account={result.get('account_id')!r})"
    )


def _engine_http_status(exc: EngineError) -> int:
    """Preserve local service admission/deadline errors at the API boundary."""
    if exc.returncode in (400, 404, 409, 422, 429, 503):
        return exc.returncode
    if exc.returncode == -1 and (exc.job_id or "still running" in str(exc)):
        return 504
    if (
        exc.returncode == -1
        and exc.command
        and exc.command[0].startswith(("http://", "https://"))
        and "outcome is unknown" not in str(exc).lower()
    ):
        # Read-only calls can safely report that the local Go service is
        # unavailable. A failed generation POST stays a gateway error when its
        # durable acceptance cannot be resolved by idempotency key.
        return 503
    return 502


def _engine_http_exception(exc: EngineError) -> HTTPException:
    """Keep accepted durable job IDs available when a compatibility wait expires."""
    headers = {}
    if exc.job_id:
        headers["X-Flow-Job-ID"] = exc.job_id
    if exc.idempotency_key:
        headers["Idempotency-Key"] = exc.idempotency_key
    return HTTPException(
        status_code=_engine_http_status(exc), detail=str(exc), headers=headers or None
    )


def _image_urls(result: dict, base_url: str = "") -> list[str]:
    """Extract signed Flow image URLs without reading image bytes."""
    urls: list[str] = []
    for item in result.get("media") or []:
        if isinstance(item, str):
            url = item
        elif isinstance(item, dict):
            url = item.get("url") or item.get("signed_url") or item.get("download_url")
        else:
            url = None
        if isinstance(url, str) and url.strip() and (url.startswith(("http://", "https://", "/")) or not result.get("files")):
            urls.append(url)

    # Keep the flat URL view as a fallback for service/CLI results produced by
    # older or alternate engine serializers. It contains the same signed Flow
    # URLs and does not reintroduce downloading.
    if not urls:
        for url in result.get("urls") or []:
            if isinstance(url, str) and url.strip():
                urls.append(url)

    # Compatibility only: legacy local-engine results carried file paths.
    # Gateway jobs normally return `media` URLs and never enter this branch.
    if not urls:
        for item in result.get("files") or []:
            path = item.get("path") if isinstance(item, dict) else item
            if isinstance(path, str) and path.strip():
                url = "/api/v1/media/" + Path(path).name
                urls.append(base_url.rstrip("/") + url if base_url else url)

    return urls


def _image_payload(result: dict, response_format: str = "url", base_url: str = "") -> dict:
    """Turn an engine result into the OpenAI images response."""
    # Image generation keeps Flow's signed URL. The Go engine deliberately does
    # not download image bytes or create local media files.
    if response_format == "b64_json":
        data = []
        for item in result.get("files") or []:
            path = item.get("path") if isinstance(item, dict) else item
            if not isinstance(path, str):
                continue
            try:
                import base64
                encoded = base64.b64encode(Path(path).read_bytes()).decode("ascii")
            except (OSError, ValueError):
                continue
            data.append({"b64_json": encoded})
        if not data:
            data = []
    else:
        data = []
        for url in _image_urls(result, base_url):
            if url.startswith("/") and base_url:
                url = base_url.rstrip("/") + url
            data.append({"url": url})

    return {"created": int(time.time()), "data": data, "flow": result}


def _prompt_from_messages(messages: list[ChatMessage]) -> str:
    """The text of the last user turn, which is what the engine generates from."""
    for message in reversed(messages):
        if message.role != "user":
            continue
        content = message.content
        if isinstance(content, str):
            return content
        # The newer OpenAI shape is a list of parts; take the text ones.
        if isinstance(content, list):
            parts = [
                part.get("text", "")
                for part in content
                if isinstance(part, dict) and part.get("type") == "text"
            ]
            joined = " ".join(p for p in parts if p).strip()
            if joined:
                return joined
    return ""


def _gemini_prompt(payload: dict[str, Any]) -> str:
    """Extract text from the common Gemini contents/parts request shape."""
    contents = payload.get("contents")
    if not isinstance(contents, list):
        raise HTTPException(status_code=400, detail="contents must be a non-empty array")
    texts: list[str] = []
    for content in contents:
        if not isinstance(content, dict):
            continue
        parts = content.get("parts")
        if not isinstance(parts, list):
            continue
        for part in parts:
            if isinstance(part, dict) and isinstance(part.get("text"), str):
                text = part["text"].strip()
                if text:
                    texts.append(text)
    prompt = "\n".join(texts).strip()
    if not prompt:
        raise HTTPException(status_code=400, detail="contents must contain a text part")
    return prompt


def _gemini_reference_data(payload: dict[str, Any]) -> list[dict[str, str]]:
    """Convert Gemini inline image parts to the Go service reference contract."""
    references: list[dict[str, str]] = []
    contents = payload.get("contents")
    if not isinstance(contents, list):
        return references
    for content in contents:
        if not isinstance(content, dict) or not isinstance(content.get("parts"), list):
            continue
        for part in content["parts"]:
            if not isinstance(part, dict):
                continue
            inline = part.get("inlineData")
            if not isinstance(inline, dict):
                continue
            data = inline.get("data")
            mime_type = inline.get("mimeType")
            if not isinstance(data, str) or not data.strip():
                raise HTTPException(status_code=400, detail="inlineData.data must be non-empty base64")
            if not isinstance(mime_type, str) or not mime_type.startswith("image/"):
                raise HTTPException(status_code=400, detail="inlineData.mimeType must be an image MIME type")
            references.append({"base64": data, "mime_type": mime_type, "file_name": "gemini-reference"})
    return references


def _gemini_video_reference(instance: dict[str, Any]) -> list[dict[str, str]]:
    """Read the official Veo image/referenceImages input shape."""
    references: list[dict[str, str]] = []
    image = instance.get("image") if isinstance(instance, dict) else None
    if isinstance(image, dict):
        inline = image.get("inlineData")
        if isinstance(inline, dict):
            data, mime_type = inline.get("data"), inline.get("mimeType")
        else:
            data, mime_type = image.get("bytesBase64Encoded"), image.get("mimeType")
        # An image URI is handled by the existing ``start_image`` argument;
        # only inline image bytes need to be forwarded through reference_data.
        if data is not None or mime_type is not None:
            if not isinstance(data, str) or not data.strip() or not isinstance(mime_type, str) or not mime_type.startswith("image/"):
                raise HTTPException(status_code=400, detail="instances[0].image must contain valid image data and mimeType")
            references.append({"base64": data, "mime_type": mime_type, "file_name": "gemini-video-image"})

    reference_images = instance.get("referenceImages")
    if reference_images is not None and not isinstance(reference_images, list):
        raise HTTPException(status_code=400, detail="instances[0].referenceImages must be an array")
    if len(reference_images or []) > 3:
        raise HTTPException(status_code=400, detail="instances[0].referenceImages supports at most 3 images")
    for index, reference in enumerate(reference_images or []):
        if not isinstance(reference, dict) or reference.get("referenceType") != "asset":
            raise HTTPException(status_code=400, detail=f"referenceImages[{index}].referenceType must be 'asset'")
        ref_image = reference.get("image")
        inline = ref_image.get("inlineData") if isinstance(ref_image, dict) else None
        if not isinstance(inline, dict):
            raise HTTPException(status_code=400, detail=f"referenceImages[{index}].image.inlineData is required")
        data, mime_type = inline.get("data"), inline.get("mimeType")
        if not isinstance(data, str) or not data.strip() or not isinstance(mime_type, str) or not mime_type.startswith("image/"):
            raise HTTPException(status_code=400, detail=f"referenceImages[{index}] must contain valid inlineData")
        references.append({"base64": data, "mime_type": mime_type, "file_name": f"gemini-reference-{index + 1}"})
    return references


def _gemini_aspect(config: dict[str, Any]) -> str:
    image_config = config.get("imageConfig")
    ratio = config.get("aspectRatio")
    if ratio is None and isinstance(image_config, dict):
        ratio = image_config.get("aspectRatio")
    ratio = str(ratio or "1:1").strip().lower()
    return {"square": "1:1", "landscape": "16:9", "portrait": "9:16"}.get(ratio, ratio)


def _gemini_image_model(model: str) -> str:
    try:
        return GEMINI_IMAGE_MODELS[model.lower()]
    except KeyError as exc:
        raise HTTPException(
            status_code=400,
            detail=f"unsupported Gemini image model {model!r}; use one of {sorted(GEMINI_IMAGE_MODELS)}",
        ) from exc


def _gemini_video_model(model: str, duration: str, has_references: bool, has_start_image: bool) -> tuple[str, str]:
    try:
        mapped = GEMINI_VIDEO_MODELS[model.lower()]
    except KeyError as exc:
        raise HTTPException(
            status_code=400,
            detail=f"unsupported Gemini video model {model!r}; use one of {sorted(GEMINI_VIDEO_MODELS)}",
        ) from exc
    # Omni is an explicit Flow video model. Preserve it exactly so Go can send
    # the requested key instead of replacing it with a duration family.
    if mapped == "omni_1_1_flash":
        return mapped, "720p"
    quality = mapped
    # The current Go reference-video catalog has no verified 360p variant;
    # keep reference-image jobs on the catalogued 720p key.
    if has_references:
        quality = "720p"
    seconds = int(duration)
    if has_references:
        family = "abra_r2v"
    elif has_start_image:
        family = "abra_i2v"
    else:
        family = "abra_t2v"
    return f"{family}_{seconds}s", quality


def _gemini_image_response(model: str, result: dict) -> dict:
    urls = _image_urls(result)
    if not urls:
        raise HTTPException(status_code=502, detail=_no_image_detail(result))
    parts = [
        {"fileData": {"mimeType": "image/png", "fileUri": url}}
        for url in urls
    ]
    return {
        "candidates": [{
            "content": {"role": "model", "parts": parts},
            "finishReason": "STOP",
        }],
        "modelVersion": model,
        "usageMetadata": {"promptTokenCount": 0, "candidatesTokenCount": 0, "totalTokenCount": 0},
        "flow": result,
    }


def _gemini_video_operation(model: str, result: dict, operation_name: Optional[str] = None) -> dict:
    urls = _image_urls(result)
    if not urls:
        raise HTTPException(status_code=502, detail="the engine produced no video")
    generated = [{"video": {"uri": url, "mimeType": "video/mp4"}} for url in urls]
    return {
        "name": operation_name or f"operations/flow-{result.get('job_id', int(time.time() * 1000))}",
        "done": True,
        "response": {"generateVideoResponse": {"generatedSamples": generated}},
        "metadata": {"model": model, "flow": result},
    }


def _gemini_operation_name(job_id: str) -> str:
    return f"operations/flow-{job_id}"


def _gemini_job_id(operation_id: str) -> str:
    prefix = "flow-"
    if not operation_id.startswith(prefix) or not operation_id[len(prefix):].strip():
        raise HTTPException(status_code=404, detail="operation not found")
    return operation_id[len(prefix):]


def _completion_body(model: str, markdown: str) -> dict:
    return {
        "id": f"chatcmpl-flow-{int(time.time() * 1000)}",
        "object": "chat.completion",
        "created": int(time.time()),
        "model": model,
        "choices": [
            {
                "index": 0,
                "message": {"role": "assistant", "content": markdown},
                "finish_reason": "stop",
            }
        ],
        "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
    }


def _sse(body: dict) -> StreamingResponse:
    """Stream one completion as the two SSE frames an OpenAI client expects.

    The content is delivered in one delta rather than token by token: the engine
    produces an image, not prose, so there is nothing to stream progressively —
    but a client that asked for `stream: true` still needs the framing.
    """

    async def frames():
        head = {
            "id": body["id"],
            "object": "chat.completion.chunk",
            "created": body["created"],
            "model": body["model"],
            "choices": [
                {"index": 0, "delta": {"role": "assistant"}, "finish_reason": None}
            ],
        }
        yield f"data: {json.dumps(head)}\n\n"

        chunk = {
            **head,
            "choices": [
                {
                    "index": 0,
                    "delta": {"content": body["choices"][0]["message"]["content"]},
                    "finish_reason": None,
                }
            ],
        }
        yield f"data: {json.dumps(chunk)}\n\n"

        tail = {
            **head,
            "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}],
        }
        yield f"data: {json.dumps(tail)}\n\n"
        yield "data: [DONE]\n\n"

    return StreamingResponse(frames(), media_type="text/event-stream")


# --------------------------------------------------------------------------- #
# OpenAI-compatible
# --------------------------------------------------------------------------- #


@app.post("/v1/images/generations")
async def openai_images(
    payload: ImageGenerationRequest, request: Request,
    idempotency_key: Optional[str] = Header(default=None, alias="Idempotency-Key"),
):
    """Generate an image from an OpenAI images payload."""
    aspect = SIZE_TO_ASPECT.get(payload.size.strip().lower())
    if aspect is None:
        raise HTTPException(
            status_code=400,
            detail=(
                f"unsupported size {payload.size!r}; use one of "
                f"{', '.join(sorted(SIZE_TO_ASPECT))}"
            ),
        )

    try:
        retry_key = {"idempotency_key": idempotency_key} if idempotency_key else {}
        result = await engine.generate_image(
            prompt=payload.prompt,
            aspect=aspect,
            count=payload.n,
            model=payload.model,
            cookies=payload.cookies,
            **retry_key,
        )
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc

    payload_out = _image_payload(result, payload.response_format, str(request.base_url))
    # A run that produced nothing is not a success. Returning 200 with an empty
    # `data` would tell an OpenAI client the request worked and there simply
    # were no images, which is the opposite of what happened.
    if not payload_out["data"]:
        raise HTTPException(status_code=502, detail=_no_image_detail(result))

    return payload_out


@app.post("/v1/chat/completions")
async def openai_chat(
    payload: ChatCompletionRequest, request: Request,
    idempotency_key: Optional[str] = Header(default=None, alias="Idempotency-Key"),
):
    """Generate from the last user turn and answer with a Markdown link."""
    prompt = _prompt_from_messages(payload.messages)
    if not prompt:
        raise HTTPException(status_code=400, detail="no user message with text to generate from")

    aspect = SIZE_TO_ASPECT.get(payload.size.strip().lower(), "1:1")

    try:
        retry_key = {"idempotency_key": idempotency_key} if idempotency_key else {}
        result = await engine.generate_image(
            prompt=prompt,
            aspect=aspect,
            cookies=payload.cookies,
            **retry_key,
        )
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc

    image_urls = _image_urls(result, str(request.base_url))
    if not image_urls:
        markdown = _no_image_detail(result)
    else:
        links = "\n".join(
            f"![generated image]({url})" for url in image_urls
        )
        markdown = links

    body = _completion_body(payload.model, markdown)
    return _sse(body) if payload.stream else body


# --------------------------------------------------------------------------- #
# Gemini-compatible facade
# --------------------------------------------------------------------------- #


@app.post("/v1beta/models/{model}:generateContent")
@app.post("/v1/models/{model}:generateContent")
async def gemini_generate_content(
    model: str,
    payload: dict[str, Any],
    idempotency_key: Optional[str] = Header(default=None, alias="Idempotency-Key"),
):
    """Accept the common Gemini image-generation request shape.

    This is deliberately an adapter, not a second upstream implementation.
    Gemini model names are accepted for client compatibility; Flow's existing
    image model selection remains in the Go-backed engine.
    """
    prompt = _gemini_prompt(payload)
    internal_model = _gemini_image_model(model)
    config = payload.get("generationConfig")
    if not isinstance(config, dict):
        config = {}
    count = config.get("candidateCount", 1)
    if isinstance(count, bool) or not isinstance(count, int) or not 1 <= count <= 4:
        raise HTTPException(status_code=400, detail="generationConfig.candidateCount must be 1..4")
    try:
        retry_key = {"idempotency_key": idempotency_key} if idempotency_key else {}
        reference_data = _gemini_reference_data(payload)
        reference_kwargs = {"reference_data": reference_data} if reference_data else {}
        result = await engine.generate_image(
            prompt=prompt,
            aspect=_gemini_aspect(config),
            count=count,
            model=internal_model,
            **reference_kwargs,
            **retry_key,
        )
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc
    return _gemini_image_response(model, result)


@app.post("/v1beta/models/{model}:predictLongRunning")
@app.post("/v1/models/{model}:predictLongRunning")
async def gemini_generate_videos(
    model: str,
    payload: GeminiVideoRequest,
    idempotency_key: Optional[str] = Header(default=None, alias="Idempotency-Key"),
):
    """Submit a Gemini Veo long-running operation without waiting for the video."""
    instance = payload.instances[0] if payload.instances else {}
    prompt = instance.get("prompt") if isinstance(instance, dict) else ""
    if not isinstance(prompt, str) or not prompt.strip():
        raise HTTPException(status_code=400, detail="instances[0].prompt is required")
    params = payload.parameters
    duration = params.get("durationSeconds", "8")
    if duration not in ("4", "6", "8"):
        raise HTTPException(status_code=400, detail="parameters.durationSeconds must be one of '4', '6', or '8'")
    duration_text = f"{duration}s"
    quality = str(params.get("resolution", "720p"))
    count = params.get("numberOfVideos", 1)
    if isinstance(count, bool) or not isinstance(count, int) or not 1 <= count <= 4:
        raise HTTPException(status_code=400, detail="parameters.numberOfVideos must be an integer from 1 to 4")
    start_image = None
    image = instance.get("image") if isinstance(instance, dict) else None
    if isinstance(image, dict):
        start_image = image.get("uri")
    reference_data = _gemini_video_reference(instance)
    internal_model, model_quality = _gemini_video_model(
        model,
        duration,
        has_references=bool(reference_data),
        has_start_image=bool(start_image),
    )
    if model.lower() in {"omni-1.1-flash", "omni_1_1_flash"} and quality in {"360p", "720p"}:
        model_quality = quality
    try:
        retry_key = {"idempotency_key": idempotency_key} if idempotency_key else {}
        reference_kwargs = {"reference_data": reference_data} if reference_data else {}
        accepted = await engine.enqueue_service_job(
            "video",
            {
                "prompt": prompt,
                "aspect": _gemini_aspect(params),
                "duration": duration_text,
                "quality": model_quality,
                "count": count,
                "model": internal_model,
                "start_image": start_image,
                **reference_kwargs,
            },
            idempotency_key,
        )
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc
    job_id = accepted.get("job_id") if isinstance(accepted, dict) else None
    if not isinstance(job_id, str) or not job_id.strip():
        raise HTTPException(status_code=502, detail="service returned no job_id")
    operation_name = _gemini_operation_name(job_id)
    return {
        "name": operation_name,
        "done": False,
        "metadata": {"model": model, "jobId": job_id},
    }


@app.get("/v1beta/operations/{operation_id}")
@app.get("/v1/operations/{operation_id}")
async def gemini_get_operation(operation_id: str):
    """Poll a Gemini operation backed by a durable Flow service job."""
    job_id = _gemini_job_id(operation_id)
    try:
        status = await engine.get_service_job(job_id)
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc

    operation_name = _gemini_operation_name(job_id)
    model = str(status.get("result", {}).get("model", "")) if isinstance(status.get("result"), dict) else ""
    if status.get("status") in {"queued", "running", "accepted", "pending"}:
        return {"name": operation_name, "done": False, "metadata": {"jobId": job_id}}
    if status.get("status") == "succeeded":
        result = status.get("result")
        if not isinstance(result, dict):
            raise HTTPException(status_code=502, detail="service returned no video result")
        try:
            return _gemini_video_operation(model, result, operation_name)
        except HTTPException:
            raise
    if status.get("status") == "partial":
        result = status.get("result")
        if isinstance(result, dict):
            return _gemini_video_operation(model, result, operation_name)
    if status.get("status") in {"failed", "cancelled", "submission_unknown"}:
        return {
            "name": operation_name,
            "done": True,
            "error": {
                "code": status.get("error_code", 502),
                "message": status.get("error") or f"job ended with status {status.get('status')}",
                "status": status.get("status"),
            },
        }
    return {"name": operation_name, "done": False, "metadata": {"jobId": job_id}}


# --------------------------------------------------------------------------- #
# native
# --------------------------------------------------------------------------- #


@app.get("/", response_class=HTMLResponse)
async def dashboard():
    dashboard_path = BASE_DIR / "dashboard.html"
    if not dashboard_path.is_file():
        dashboard_path = BASE_DIR.parent / "dashboard.html"
    return FileResponse(
        dashboard_path,
        media_type="text/html",
        headers={"Cache-Control": "no-store"},
    )


@app.post("/api/v1/jobs")
async def create_async_job(
    payload: AsyncJobRequest,
    idempotency_key: Optional[str] = Header(default=None, alias="Idempotency-Key"),
):
    """Accept asynchronous work and return immediately with a durable status URL."""
    if payload.kind in ("image", "video") and not str(payload.payload.get("prompt", "")).strip():
        raise HTTPException(status_code=422, detail="payload.prompt is required")
    if payload.kind in ("image", "video"):
        count = payload.payload.get("count", 1)
        if isinstance(count, bool) or not isinstance(count, int) or not 1 <= count <= 4:
            raise HTTPException(status_code=422, detail="payload.count must be an integer from 1 to 4")
    if payload.kind == "upload-video" and not str(payload.payload.get("file_path", "")).strip():
        raise HTTPException(status_code=422, detail="payload.file_path is required")
    try:
        accepted = await engine.enqueue_service_job(payload.kind, payload.payload, idempotency_key)
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc
    job_id = accepted.get("job_id")
    if not job_id:
        raise HTTPException(status_code=502, detail="service returned no job_id")
    status_url = f"/api/v1/jobs/{job_id}"
    events_url = f"/api/v1/jobs/{job_id}/events"
    return JSONResponse(
        status_code=202,
        headers={"Location": status_url},
        content={
            "job_id": job_id,
            "status": accepted.get("status", "queued"),
            "status_url": status_url,
            "events_url": events_url,
        },
    )


@app.post("/api/v1/uploads")
@app.post("/v1/uploads")
async def upload_video_file(
    file: UploadFile = File(...),
    force: bool = Form(default=False),
    idempotency_key: Optional[str] = Header(default=None, alias="Idempotency-Key"),
):
    """Stage a client upload briefly, then stream it to the selected Go node."""
    suffix = Path(file.filename or "upload.mp4").suffix or ".mp4"
    temp_path = ""
    try:
        with tempfile.NamedTemporaryFile(prefix="gateway-upload-", suffix=suffix, delete=False) as staged:
            temp_path = staged.name
            shutil.copyfileobj(file.file, staged)
        return await engine.upload_video(
            temp_path, force=force,
            idempotency_key=idempotency_key,
        )
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc
    finally:
        await file.close()
        if temp_path:
            try:
                Path(temp_path).unlink(missing_ok=True)
            except OSError:
                pass


@app.get("/api/v1/jobs/{job_id}")
async def get_async_job(job_id: str):
    try:
        return await engine.get_service_job(job_id)
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc


@app.get("/api/v1/jobs/{job_id}/events")
async def stream_async_job_events(
    job_id: str,
    last_event_id: Optional[str] = Header(default=None, alias="Last-Event-ID"),
):
    if isinstance(engine, GatewayEngine):
        try:
            stream = engine.stream_service_job_events(job_id, last_event_id)
        except EngineError as exc:
            raise _engine_http_exception(exc) from exc
        return StreamingResponse(
            stream,
            media_type="text/event-stream",
            headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
        )
    try:
        upstream = await engine.open_service_job_events(job_id, last_event_id)
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc

    async def stream():
        try:
            async for chunk in upstream.aiter_bytes():
                yield chunk
        finally:
            await upstream.aclose()

    return StreamingResponse(
        stream(),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )


@app.post("/api/v1/jobs/{job_id}/cancel", status_code=202)
async def cancel_async_job(job_id: str):
    try:
        await engine.cancel_service_job(job_id)
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc
    return {"job_id": job_id, "status": "cancellation_requested"}


@app.post("/v1/generate/image")
@app.post("/v1/image")
@app.post("/api/v1/image")
async def flow_image(
    payload: FlowImageRequest,
    idempotency_key: Optional[str] = Header(default=None, alias="Idempotency-Key"),
):
    """Generate an image, speaking the engine's own vocabulary."""
    try:
        retry_key = {"idempotency_key": idempotency_key} if idempotency_key else {}
        return await engine.generate_image(
            prompt=payload.prompt,
            aspect=payload.aspect,
            count=payload.count,
            model=payload.model,
            cookies=payload.cookies,
            **retry_key,
        )
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc


@app.post("/v1/generate/video")
@app.post("/v1/video")
@app.post("/api/v1/video")
async def flow_video(
    payload: FlowVideoRequest,
    idempotency_key: Optional[str] = Header(default=None, alias="Idempotency-Key"),
):
    """Generate a video, speaking the engine's own vocabulary."""
    try:
        retry_key = {"idempotency_key": idempotency_key} if idempotency_key else {}
        return await engine.generate_video(
            prompt=payload.prompt,
            aspect=payload.aspect,
            duration=payload.duration,
            quality=payload.quality,
            count=payload.count,
            start_image=payload.start_image,
            cookies=payload.cookies,
            **retry_key,
        )
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc


@app.get("/v1/balance")
@app.get("/api/v1/balance")
async def flow_balance(refresh: bool = False):
    """Credits per account, cached by default and probed with `?refresh=true`."""
    try:
        return await engine.get_balance(refresh=refresh)
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc


@app.get("/v1/stats")
@app.get("/api/v1/stats")
async def flow_stats():
    """Generation history and engine totals, read from the database."""
    try:
        return await engine.get_stats()
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc


@app.get("/api/v1/monitor")
async def flow_monitor(limit: int = 50, status: str = "", kind: str = "",
                       page: Optional[int] = None, page_size: Optional[int] = None,
                       request_page: Optional[int] = None, request_page_size: Optional[int] = None):
    """Live Go service intake, durable jobs, queue pressure and phase latency."""
    if not 1 <= limit <= 200:
        raise HTTPException(status_code=400, detail="limit must be between 1 and 200")
    if page is not None and not 0 <= page <= 1_000_000_000:
        raise HTTPException(status_code=400, detail="page must be between 0 and 1000000000")
    if page_size is not None and not 1 <= page_size <= 200:
        raise HTTPException(status_code=400, detail="page_size must be between 1 and 200")
    if request_page is not None and not 0 <= request_page <= 1_000_000_000:
        raise HTTPException(status_code=400, detail="request_page must be between 0 and 1000000000")
    if request_page_size is not None and not 1 <= request_page_size <= 200:
        raise HTTPException(status_code=400, detail="request pagination is out of range")
    try:
        pagination = {}
        if page is not None or page_size is not None:
            pagination = {"page": page if page is not None else 0,
                          "page_size": page_size if page_size is not None else 20}
        if request_page is not None or request_page_size is not None:
            pagination.update({"request_page": request_page if request_page is not None else 0,
                               "request_page_size": request_page_size if request_page_size is not None else 20})
        return await engine.get_monitor(limit=limit, status=status, kind=kind, **pagination)
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc


@app.get("/api/v1/settings")
async def get_runtime_settings():
    try:
        return await engine.get_runtime_settings()
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc


@app.patch("/api/v1/settings")
async def update_runtime_settings(request: RuntimeSettingsRequest):
    if not request.values and not request.reset:
        raise HTTPException(status_code=400, detail="provide values or reset keys")
    try:
        return await engine.update_runtime_settings(request.model_dump())
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc


@app.get("/v1/projects")
@app.get("/api/v1/projects")
async def flow_projects(cookies: Optional[str] = None):
    try:
        return {"projects": await engine.get_projects(cookies=cookies)}
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc


@app.get("/v1/media/{filename:path}")
@app.get("/api/v1/media/{filename:path}")
async def flow_media(filename: str, request: Request):
    """Proxy media from the node that owns it; Gateway has no shared disk."""
    if not isinstance(engine, GatewayEngine):
        candidate = (MEDIA_DIR / filename).resolve()
        if candidate.parent != MEDIA_DIR.resolve() or not candidate.is_file():
            raise HTTPException(status_code=404, detail="no such media file")
        return FileResponse(candidate)
    try:
        proxy_headers = {}
        for header in ("range", "if-range", "if-none-match"):
            value = request.headers.get(header)
            if value:
                proxy_headers[header.title()] = value
        upstream, metadata = await engine.open_media(filename, proxy_headers)
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc

    async def stream():
        try:
            async for chunk in upstream.aiter_bytes():
                yield chunk
        finally:
            await upstream.aclose()

    headers = {"Cache-Control": "private, max-age=3600", "X-Accel-Buffering": "no"}
    if metadata.get("file_name"):
        headers["Content-Disposition"] = f'inline; filename="{metadata["file_name"]}"'
    if metadata.get("content_type"):
        headers["Content-Type"] = metadata["content_type"]
    return StreamingResponse(stream(), headers=headers)


@app.get("/health")
async def health():
    service_health = getattr(engine, "service_health", None)
    details = await service_health() if service_health is not None else {}
    if service_health is not None:
        engine_available = details.get("status") == "ok"
    else:
        available = getattr(engine, "service_available", None)
        engine_available = await available() if available is not None else bool(engine.available())
    mode = getattr(engine, "engine_mode", "gateway")
    body = {
        "status": "ok",
        "engine": engine_available,
        "mode": mode,
    }
    for key in ("ready_accounts", "connected_accounts", "strict_cookie_freshness"):
        if key in details:
            body[key] = details[key]
    return body
