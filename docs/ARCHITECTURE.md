# Flow Agent Architecture

Comprehensive system design, component boundaries, and data flow for **Flow Agent**.

---

## 1. Overview & Design Philosophy

Flow Agent provides an OpenAI-compatible REST API, an Anthropic Model Context Protocol (MCP) server, and a unified CLI on top of Google Flow's image and video generation platform.

The system uses a persistent Go service with Python API adapters. Python retains a subprocess mode for migration and rollback.

1. **The Core Engine (`bin/flow`)**: A compiled, high-performance Go binary that handles the complex, sensitive upstream protocol interactions:
   - Reverse-engineered Google Flow `batchexecute` RPCs.
   - Session management, credential refreshing, and token minting (`f.sid`, `at`).
   - Browser fingerprint consistency and reCAPTCHA handling.
   - Multi-account pool rotation, rate limiting, and pacing.
   - Direct SQLite transactions (`data/flow.db`).
2. **The Application Layer (`flow_agent/`)**: A clean, asynchronous Python package that exposes the engine to modern development stacks:
   - **FastAPI Surface (`api.py`)**: Native Flow routes + 100% OpenAI-compatible endpoints (`/v1/images/generations`, `/v1/chat/completions`).
   - **MCP Server (`mcp_server.py`)**: Standards-compliant JSON-RPC 2.0 server supporting both Stdio and Server-Sent Events (SSE).
   - **Service Adapter (`engine.py`)**: Submits durable jobs to the Go service and polls results in explicit service mode. It retains the CLI subprocess adapter when selected for migration and rollback.

```mermaid
flowchart TD
    subgraph Clients["Clients & Consumers"]
        OpenAI["OpenAI SDK / Chat UIs\n(LangChain, Next.js, LlamaIndex)"]
        MCPClient["MCP Clients\n(Claude Desktop, Cursor, Zed)"]
        Browser["Chrome Browser\n(Google Flow Tab)"]
        Terminal["Developer CLI\n(main.py / flow-agent)"]
    end

    subgraph AppLayer["Python Application Layer (flow_agent/)"]
        FastAPI["FastAPI Application\n(Native + OpenAI Routes)"]
        MCPServer["MCP Server\n(Stdio & SSE Transports)"]
        CLI["CLI Parser & Subcommands\n(main.py)"]
        EngineBridge["Async FlowEngine Bridge\n(engine.py)"]
    end

    subgraph CoreEngine["Persistent Go Service & Storage"]
        Binary["flow-go service\n(bridge + account runtimes + scheduler)"]
        Queue["SQLite durable jobs\n(idempotency + leases + recovery)"]
        Cookies["Account Bundles\n(cookies/account_*.json)"]
        Database["SQLite Database\n(data/flow.db)"]
        MediaOutput["Asset Storage\n(output/*.jpg, *.mp4)"]
    end

    OpenAI -->|HTTP / SSE| FastAPI
    MCPClient -->|JSON-RPC (Stdio/SSE)| MCPServer
    Terminal -->|Argv| CLI
    Browser -->|WebSocket ws://127.0.0.1:9222| Binary

    FastAPI --> EngineBridge
    MCPServer --> EngineBridge
    CLI --> EngineBridge

    EngineBridge -->|Authenticated job API| Binary
    EngineBridge -->|Read-Only SQLite queries| Database
    Binary --> Queue
    Queue --> Binary
    Binary -->|Warm runtime per account| Cookies
    Binary -->|ACID Transactions| Database
    Binary -->|Save Media| MediaOutput
    FastAPI -->|Serve Media| MediaOutput
```

---

## 2. Directory Layout & Roles

```text
flow-agent/
├── bin/
│   └── flow                     # Statically compiled Go engine binary
├── cookies/
│   └── account_<hash>.json      # Account credentials, cookies, tokens & fingerprints
├── data/
│   └── flow.db                  # SQLite database (accounts, generations, media history)
├── output/
│   └── *.jpg, *.mp4             # Downloaded generated media assets
├── flow_agent/                  # Python Package
│   ├── __init__.py              # Package exports (FlowEngine, app, create_sse_app, etc.)
│   ├── __main__.py              # Module entrypoint (`python -m flow_agent`)
│   ├── engine.py                # Async subprocess bridge & read-only SQLite connector
│   ├── api.py                   # FastAPI REST API (Native + OpenAI routes)
│   └── mcp_server.py            # MCP Server (JSON-RPC 2.0 over Stdio & SSE)
├── tests/                       # Test Suite (74 tests, 100% offline)
│   ├── __init__.py
│   ├── conftest.py              # Pytest path & environment configuration
│   ├── test_api.py              # API route tests with engine stubs
│   ├── test_engine.py           # Subprocess & extraction unit tests
│   ├── test_main.py             # CLI parser tests
│   └── test_mcp.py              # Pure protocol JSON-RPC tests
├── docs/                        # Complete System Documentation
│   ├── ARCHITECTURE.md          # Architecture & system design
│   ├── API_REFERENCE.md         # Full REST API endpoints & schemas
│   ├── MCP_GUIDE.md             # MCP tool schemas & client setup
│   ├── EXTENSION_BRIDGE.md      # Extension WebSocket bridge specification
│   └── CLI_REFERENCE.md         # Command-line interface guide
├── main.py                      # Unified CLI entrypoint
├── pyproject.toml               # PEP 621 package metadata & dependencies
└── README.md                    # Quick-start and usage overview
```

---

## 3. Component Details

### 3.1. Persistent Go Service
- `flow-go service` owns the WebSocket bridge, SQLite queue, account registry and job workers. It listens on `127.0.0.1:9223` by default; the extension continues to use `127.0.0.1:9222`.
- The service loads one isolated Go engine runtime per complete `cookies/account_<key>.json` bundle. Each runtime keeps its own cookies, project, page metadata, captcha strategy and pacing state.
- Before assigning a live account to a job, the service pulls a full scoped cookie snapshot from that account's extension. Rotation events invalidate the old runtime immediately without waiting for bootstrap; per-account revisions prevent an older in-progress bootstrap from publishing after a newer event. Atomic bundle replacement and runtime reloads keep account state aligned. Account views sharing one CDP client serialize operations against its current tab, including project navigation/discovery, page metadata, cookie refresh and browser-backed captcha; independent CDP clients run concurrently.
- Jobs are persisted before acknowledgement. Idempotency keys, worker leases, per-execution attempt history and bounded admission (1,000 outstanding jobs) protect the queue. Recovery requeues safe pre-submit work, restores saved completed results, and resumes accepted media polling/download from stored account, project and media IDs without resubmitting generation. Expired execution with no durable media IDs becomes `submission_unknown`. Terminal job records are removed after 30 days; `POST /internal/v1/jobs/{id}/cancel` cancels queued or active work.
- The scheduler runs one generation at a time per account by default, distributes jobs across idle accounts and caps total active jobs with `--workers`. Throttle, out-of-credit and circuit cooldown deadlines are persisted per account in SQLite and restored on service startup; refreshing a positive authoritative balance clears only an out-of-credit cooldown. `all_accounts` snapshots eligible accounts at acceptance and atomically stores one durable child job per account; image `count` similarly creates pinned child jobs. Parent status and aggregate results survive restart and preserve partial failures.
- Locally known media IDs are checked against their producing account and project before use. Upload-cache keys include account and project. Known references pin to their owner; opaque external media IDs require an explicit `account_id`, while local image files can be re-uploaded by another eligible account on a safe pre-submit retry. Account throttles return to the durable queue with a separate `FLOW_JOB_MAX_THROTTLE_ATTEMPTS` budget.
- The internal API requires `Authorization: Bearer <FLOW_INTERNAL_TOKEN>`. Keep it on loopback unless a trusted network boundary protects it.

### 3.2. Python Adapter (`flow_agent/engine.py`)
- Set `FLOW_ENGINE_MODE=service`, `FLOW_SERVICE_URL=http://127.0.0.1:9223` and the same `FLOW_INTERNAL_TOKEN` in the Python process to use the persistent service. Python submits each job once and polls its durable status. Setting a service URL without an explicit mode fails startup rather than silently selecting an execution path.
- Set `FLOW_ENGINE_MODE=subprocess` explicitly to select the prior CLI subprocess mode and `extract_json()` behavior for migration and rollback. Startup fails when `FLOW_ENGINE_MODE` is unset, preventing a silent fallback to one subprocess per request.
- In service mode, balance refresh, project reads and media generation use the Go service. Generation statistics still read the shared local SQLite database.

### 3.3. REST API Layer (`flow_agent/api.py`)
- **Native Routes**: Speak Flow's exact domain vocabulary (`aspect`, `duration`, `quality`, `model`).
- **OpenAI Compatibility**:
  - `/v1/images/generations`: Translates standard OpenAI image requests (including sizes like `1024x1024`, `1792x1024`, `1024x1792` and aliases `square`, `landscape`, `portrait`). Supports both `url` and `b64_json` response formats.
  - `/v1/chat/completions`: Extracts prompts from the latest user message and responds with markdown image links. Supports real-time Server-Sent Events (`stream: true`).
- **Security**: Media serving endpoint (`/api/v1/media/{filename}`) enforces strict parent-directory resolution checks to prevent path traversal attacks (`../`).

### 3.4. MCP Protocol Layer (`flow_agent/mcp_server.py`)
- **Protocol Version**: Compliant with Anthropic MCP specification (`2024-11-05`).
- **Functional Protocol Core**: `handle_message` is implemented as a deterministic, pure function without transport coupling, enabling comprehensive unit testing without network mocks.
- **Transports**:
  - **Stdio**: Standard input/output for desktop AI agents (Claude Desktop, Cursor, Antigravity).
  - **SSE**: Server-Sent Events over HTTP for browser and cloud consumers.
- **Available Tools**:
  - `flow_generate_image`: Text-to-image with multi-aspect and multi-account support.
  - `flow_generate_video`: Text-to-video, image-to-video with duration and quality controls.
  - `flow_get_balance`: Live or cached credit checks across all accounts.
  - `flow_get_stats`: Generation analytics and status totals.
  - `flow_upload_video`: Video asset upload and media ID registration.

---

## 4. Multi-Account Management & Persistence

1. **Account Credentials**: Each account is saved as a discrete JSON file in `cookies/account_<hash>.json`.
2. **Account Switching**: When generating assets, callers can:
   - Target a specific account via `--cookies cookies/account_<id>.json`.
   - Distribute jobs across all signed-in accounts simultaneously via `--all`.
   - Default to automatic round-robin across active accounts with positive credit balance.
3. **Database Schema (`data/flow.db`)**:
   - `accounts`: Account ID, project ID, cached credits, status (`active`/`disabled`).
   - `generations`: Job ID, prompt, kind (`image`/`video`), aspect, duration, status, elapsed milliseconds, timestamp.
   - `media`: Mapping between media IDs, file paths, hashes, and remote Flow asset IDs.
