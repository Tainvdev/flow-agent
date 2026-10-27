"""Public package facade without eagerly starting the legacy CLI/MCP engine."""

from .api import app


def __getattr__(name):
    if name in {"EngineError", "FlowEngine", "extract_json"}:
        from .engine import EngineError, FlowEngine, extract_json
        return {"EngineError": EngineError, "FlowEngine": FlowEngine,
                "extract_json": extract_json}[name]
    if name in {"create_sse_app", "run_stdio"}:
        from .mcp_server import create_sse_app, run_stdio
        return {"create_sse_app": create_sse_app, "run_stdio": run_stdio}[name]
    raise AttributeError(name)

__version__ = "1.0.0"

__all__ = [
    "FlowEngine",
    "EngineError",
    "extract_json",
    "app",
    "create_sse_app",
    "run_stdio",
    "__version__",
]
