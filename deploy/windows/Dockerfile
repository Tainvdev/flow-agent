# syntax=docker/dockerfile:1.7
# Windows deployment: Go/Python run in Docker; Chrome runs on the Windows host.

ARG GO_VERSION=1.26.4
ARG PYTHON_VERSION=3.12

FROM golang:${GO_VERSION}-bookworm AS go-builder
WORKDIR /src
COPY flow-agent/bin/flow-go/flow-go/go.mod flow-agent/bin/flow-go/flow-go/go.sum ./
RUN go mod download
COPY flow-agent/bin/flow-go/flow-go/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/flow .

FROM python:${PYTHON_VERSION}-slim-bookworm AS python-builder
ENV PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_NO_CACHE_DIR=1
WORKDIR /src/flow-agent
COPY flow-agent/pyproject.toml ./
COPY README.md ./README.md
COPY flow-agent/main.py ./main.py
COPY flow-agent/flow_agent ./flow_agent
RUN python -m venv /opt/venv \
    && /opt/venv/bin/pip install --upgrade pip \
    && /opt/venv/bin/pip install .

FROM python:${PYTHON_VERSION}-slim-bookworm AS runtime

ENV DEBIAN_FRONTEND=noninteractive \
    HOME=/home/flow \
    PATH=/opt/venv/bin:$PATH \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    FLOW_ENGINE_MODE=service \
    FLOW_SERVICE_URL=http://127.0.0.1:9223 \
    FLOW_BRIDGE_LISTEN_ADDR=0.0.0.0:9222 \
    FLOW_DATA_DIR=/var/lib/flow/data \
    FLOW_COOKIE_DIR=/var/lib/flow/cookies \
    FLOW_OUTPUT_DIR=/var/lib/flow/output \
    FLOW_COOKIE_FRESHNESS=strict \
    FLOW_SERVICE_WORKERS=8 \
    FLOW_API_PORT=8001

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash ca-certificates curl supervisor \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 flow \
    && useradd --uid 10001 --gid 10001 --create-home --shell /bin/bash flow \
    && install -d -o flow -g flow \
        /var/lib/flow/data /var/lib/flow/cookies /var/lib/flow/output \
        /var/log/supervisor /run/flow

COPY --from=go-builder /out/flow /usr/local/bin/flow
COPY --from=python-builder /opt/venv /opt/venv
COPY deploy/windows/supervisord.conf /etc/supervisor/conf.d/flow.conf
COPY deploy/windows/entrypoint.sh deploy/windows/healthcheck.sh /usr/local/bin/

RUN chmod 0755 /usr/local/bin/flow /usr/local/bin/entrypoint.sh \
        /usr/local/bin/healthcheck.sh \
    && chown -R flow:flow /opt/venv

VOLUME ["/var/lib/flow"]
EXPOSE 8001 9222

HEALTHCHECK --interval=1h --timeout=5s --start-period=30s --retries=5 \
    CMD ["/usr/local/bin/healthcheck.sh"]

STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
