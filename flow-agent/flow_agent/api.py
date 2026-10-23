"""FastAPI application: a native Flow API and an OpenAI-compatible one.

Two surfaces over one engine. The native routes speak the engine's own
vocabulary — aspect ratios, durations, quality keys — and the OpenAI routes
translate the shape an OpenAI client already sends, so an existing SDK or a chat
UI works without knowing anything about Flow.
"""

from __future__ import annotations

from contextlib import asynccontextmanager
import json
import re
import sqlite3
import time
from pathlib import Path
from typing import Any, Literal, Optional

from fastapi import FastAPI, Header, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse, HTMLResponse, JSONResponse, StreamingResponse
from pydantic import BaseModel, ConfigDict, Field

from .engine import EngineError, FlowEngine

engine = FlowEngine()


@asynccontextmanager
async def lifespan(_app: FastAPI):
    try:
        yield
    finally:
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

MEDIA_DIR = engine.output_dir
BASE_DIR = Path(__file__).resolve().parent
IDENTIFIER = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


def _database_connection() -> sqlite3.Connection:
    """Open the configured database read-only for the operator dashboard."""
    connect = getattr(engine, "_connect", None)
    if connect is not None:
        return connect()
    db_path = Path(getattr(engine, "db_path", ""))
    return sqlite3.connect(f"file:{db_path}?mode=ro", uri=True, timeout=5)


def _json_value(value: Any) -> Any:
    if isinstance(value, bytes):
        return {"type": "bytes", "hex": value.hex()}
    return value


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
    all_accounts: bool = False
    cookies: Optional[str] = None
    account_id: Optional[str] = None


class FlowImageRequest(BaseModel):
    prompt: str
    aspect: str = "1:1"
    count: int = Field(default=1, ge=1, le=4)
    model: str = "narwhal"
    all_accounts: bool = False
    cookies: Optional[str] = None
    account_id: Optional[str] = None


class FlowVideoRequest(BaseModel):
    prompt: str
    aspect: str = "landscape"
    duration: str = "8s"
    quality: str = "720p"
    count: int = Field(default=1, ge=1, le=4)
    start_image: Optional[str] = None
    all_accounts: bool = False
    cookies: Optional[str] = None
    account_id: Optional[str] = None


class AsyncJobRequest(BaseModel):
    kind: Literal["image", "video", "upload-video"]
    payload: dict[str, Any]


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
    all_accounts: bool = False
    cookies: Optional[str] = None
    account_id: Optional[str] = None


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


def _image_urls(result: dict) -> list[str]:
    """Extract signed Flow image URLs without reading image bytes."""
    urls: list[str] = []
    for item in result.get("media") or []:
        if isinstance(item, str):
            url = item
        elif isinstance(item, dict):
            url = item.get("url") or item.get("signed_url") or item.get("download_url")
        else:
            url = None
        if isinstance(url, str) and url.strip():
            urls.append(url)

    # Keep the flat URL view as a fallback for service/CLI results produced by
    # older or alternate engine serializers. It contains the same signed Flow
    # URLs and does not reintroduce downloading.
    if not urls:
        for url in result.get("urls") or []:
            if isinstance(url, str) and url.strip():
                urls.append(url)

    return urls


def _image_payload(result: dict) -> dict:
    """Turn an engine result into the OpenAI images response."""
    # Image generation keeps Flow's signed URL. The Go engine deliberately does
    # not download image bytes or create local media files.
    data = [{"url": url} for url in _image_urls(result)]

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
            all_accounts=payload.all_accounts,
            cookies=payload.cookies,
            account_id=payload.account_id,
            **retry_key,
        )
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc

    payload_out = _image_payload(result)
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
            all_accounts=payload.all_accounts,
            cookies=payload.cookies,
            account_id=payload.account_id,
            **retry_key,
        )
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc

    image_urls = _image_urls(result)
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
# native
# --------------------------------------------------------------------------- #


@app.get("/", response_class=HTMLResponse)
async def dashboard():
    return FileResponse(BASE_DIR / "dashboard.html", media_type="text/html")


@app.get("/api/v1/db/tables")
async def database_tables():
    """List SQLite tables and row counts without exposing write access."""
    try:
        with _database_connection() as conn:
            tables = conn.execute(
                "SELECT name FROM sqlite_master WHERE type='table' "
                "AND name NOT LIKE 'sqlite_%' ORDER BY name"
            ).fetchall()
            result = []
            for (name,) in tables:
                quoted = '"' + name.replace('"', '""') + '"'
                count = conn.execute(f"SELECT COUNT(*) FROM {quoted}").fetchone()[0]
                result.append({"name": name, "count": count})
        return {"database": str(getattr(engine, "db_path", "")), "tables": result}
    except (sqlite3.Error, OSError) as exc:
        raise HTTPException(status_code=503, detail=f"database unavailable: {exc}") from exc


@app.get("/api/v1/db/tables/{table_name}")
async def database_table(table_name: str, page: int = 0, page_size: int = 50, q: str = ""):
    """Read one SQLite table with bounded pagination and optional text filter."""
    if not IDENTIFIER.fullmatch(table_name):
        raise HTTPException(status_code=400, detail="invalid table name")
    if page < 0 or not 1 <= page_size <= 200:
        raise HTTPException(status_code=400, detail="page must be >= 0 and page_size must be 1..200")
    quoted = '"' + table_name.replace('"', '""') + '"'
    try:
        with _database_connection() as conn:
            exists = conn.execute(
                "SELECT 1 FROM sqlite_master WHERE type='table' AND name=?", (table_name,)
            ).fetchone()
            if not exists:
                raise HTTPException(status_code=404, detail="table not found")
            info = conn.execute(f"PRAGMA table_info({quoted})").fetchall()
            columns = [row[1] for row in info]
            where, args = "", []
            if q.strip() and columns:
                safe_columns = [f'"{c.replace(chr(34), chr(34) * 2)}"' for c in columns]
                where = " WHERE " + " OR ".join(f"CAST({c} AS TEXT) LIKE ?" for c in safe_columns)
                args = [f"%{q}%"] * len(columns)
            total = conn.execute(f"SELECT COUNT(*) FROM {quoted}{where}", args).fetchone()[0]
            order = " ORDER BY rowid DESC" if any(row[5] for row in info) else ""
            rows = conn.execute(f"SELECT * FROM {quoted}{where}{order} LIMIT ? OFFSET ?", [*args, page_size, page * page_size]).fetchall()
            return {"database": str(getattr(engine, "db_path", "")), "table": table_name, "columns": columns, "rows": [[_json_value(v) for v in row] for row in rows], "page": page, "page_size": page_size, "total": total}
    except HTTPException:
        raise
    except (sqlite3.Error, OSError) as exc:
        raise HTTPException(status_code=503, detail=f"database unavailable: {exc}") from exc


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
            all_accounts=payload.all_accounts,
            cookies=payload.cookies,
            account_id=payload.account_id,
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
            all_accounts=payload.all_accounts,
            cookies=payload.cookies,
            account_id=payload.account_id,
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


@app.get("/v1/projects")
@app.get("/api/v1/projects")
async def flow_projects(cookies: Optional[str] = None, account_id: Optional[str] = None):
    try:
        return {"projects": await engine.get_projects(cookies=cookies, account_id=account_id)}
    except EngineError as exc:
        raise _engine_http_exception(exc) from exc


@app.get("/v1/media/{filename:path}")
@app.get("/api/v1/media/{filename:path}")
async def flow_media(filename: str):
    """Serve one generated file.

    The name is resolved against the output directory and rejected if it escapes
    it: this route takes a path from a URL, and `..` in one is the whole attack.
    """
    candidate = (MEDIA_DIR / filename).resolve()
    if candidate.parent != MEDIA_DIR.resolve() or not candidate.is_file():
        raise HTTPException(status_code=404, detail="no such media file")
    return FileResponse(candidate)


@app.get("/health")
async def health():
    service_health = getattr(engine, "service_health", None)
    details = await service_health() if service_health is not None else {}
    if service_health is not None:
        engine_available = details.get("status") == "ok"
    else:
        available = getattr(engine, "service_available", None)
        engine_available = await available() if available is not None else bool(engine.available())
    service_url = getattr(engine, "service_url", "")
    body = {
        "status": "ok",
        "engine": engine_available,
        "mode": "service" if service_url else "subprocess",
        "binary": str(engine.binary),
    }
    for key in ("ready_accounts", "connected_accounts", "strict_cookie_freshness"):
        if key in details:
            body[key] = details[key]
    return body
