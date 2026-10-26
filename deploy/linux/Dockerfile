# syntax=docker/dockerfile:1.7
# Linux all-in-one deployment (Chromium + Xvfb + noVNC).
# For Windows host Chrome use deploy/windows/Dockerfile instead.

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
# pyproject.toml names README.md, while the repository keeps the canonical
# document at its root.
COPY README.md ./README.md
COPY flow-agent/main.py ./main.py
COPY flow-agent/flow_agent ./flow_agent
RUN python -m venv /opt/venv \
    && /opt/venv/bin/pip install --upgrade pip \
    && /opt/venv/bin/pip install .

FROM python:${PYTHON_VERSION}-slim-bookworm AS runtime

ENV DEBIAN_FRONTEND=noninteractive \
    DISPLAY=:99 \
    HOME=/home/flow \
    PATH=/opt/venv/bin:$PATH \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    FLOW_ENGINE_MODE=service \
    FLOW_SERVICE_URL=http://127.0.0.1:9223 \
    FLOW_DATA_DIR=/var/lib/flow/data \
    FLOW_COOKIE_DIR=/var/lib/flow/cookies \
    FLOW_OUTPUT_DIR=/var/lib/flow/output \
    FLOW_COOKIE_FRESHNESS=strict \
    FLOW_BROWSER_PROFILES=profile-1 \
    FLOW_SERVICE_WORKERS=8 \
    FLOW_API_PORT=8001 \
    FLOW_NOVNC_PORT=6080 \
    FLOW_SCREEN_SIZE=1280x800x24

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash ca-certificates chromium chromium-sandbox curl dbus-x11 fonts-dejavu-core \
        fonts-liberation novnc openbox procps supervisor websockify \
        x11vnc xvfb \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 flow \
    && useradd --uid 10001 --gid 10001 --create-home --shell /bin/bash flow \
    && install -d -o flow -g flow \
        /var/lib/flow/data /var/lib/flow/cookies /var/lib/flow/output \
        /var/lib/flow/browser /var/log/supervisor /run/flow

COPY --from=go-builder /out/flow /usr/local/bin/flow
COPY --from=python-builder /opt/venv /opt/venv
COPY flow-extension /opt/flow-extension
COPY deploy/linux/supervisord.conf /etc/supervisor/conf.d/flow.conf
COPY deploy/linux/entrypoint.sh deploy/linux/start-browsers.sh deploy/linux/start-novnc.sh deploy/linux/healthcheck.sh /usr/local/bin/

RUN chmod 0755 /usr/local/bin/flow /usr/local/bin/entrypoint.sh \
        /usr/local/bin/start-browsers.sh /usr/local/bin/start-novnc.sh \
        /usr/local/bin/healthcheck.sh \
    && chown -R flow:flow /opt/flow-extension /opt/venv

VOLUME ["/var/lib/flow"]
EXPOSE 8001 6080

HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=5 \
    CMD ["/usr/local/bin/healthcheck.sh"]

STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

