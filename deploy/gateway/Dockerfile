# Gateway-only image. Go services, browsers, extensions and node data run on
# their own machines and are never copied into this container.
FROM python:3.12-slim-bookworm

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PATH=/opt/venv/bin:$PATH \
    FLOW_API_PORT=8001

WORKDIR /app
COPY flow-agent/pyproject.toml ./flow-agent/pyproject.toml
COPY README.md ./README.md
COPY flow-agent/main.py ./flow-agent/main.py
COPY flow-agent/flow_agent ./flow-agent/flow_agent
COPY flow-agent/dashboard.html ./flow-agent/flow_agent/dashboard.html

RUN python -m venv /opt/venv \
    && /opt/venv/bin/pip install --upgrade pip \
    && /opt/venv/bin/pip install ./flow-agent

EXPOSE 8001
CMD ["/opt/venv/bin/flow-agent", "server", "--host", "0.0.0.0", "--port", "8001"]
