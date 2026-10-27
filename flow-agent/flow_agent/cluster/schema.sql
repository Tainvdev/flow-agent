CREATE TABLE IF NOT EXISTS nodes (
    node_id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    advertise_url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'registering',
    version TEXT NOT NULL DEFAULT '',
    capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,
    max_jobs INTEGER NOT NULL DEFAULT 1,
    active_jobs INTEGER NOT NULL DEFAULT 0,
    queued_jobs INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_heartbeat_at TIMESTAMPTZ,
    lease_expires_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS node_accounts (
    node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    credits INTEGER,
    ready BOOLEAN NOT NULL DEFAULT false,
    connected BOOLEAN NOT NULL DEFAULT false,
    cooldown_until TIMESTAMPTZ,
    project_id TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (node_id, account_id)
);

CREATE INDEX IF NOT EXISTS idx_node_accounts_ready
    ON node_accounts(account_id, ready, connected);

CREATE TABLE IF NOT EXISTS gateway_jobs (
    job_id UUID PRIMARY KEY,
    kind TEXT NOT NULL,
    payload JSONB NOT NULL,
    payload_version INTEGER NOT NULL DEFAULT 1,
    idempotency_key TEXT NOT NULL UNIQUE,
    requested_account TEXT NOT NULL DEFAULT '',
    node_id TEXT REFERENCES nodes(node_id),
    node_job_id TEXT,
    status TEXT NOT NULL DEFAULT 'routing',
    result JSONB,
    error TEXT NOT NULL DEFAULT '',
    error_code INTEGER NOT NULL DEFAULT 0,
    routing_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    accepted_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_gateway_jobs_status ON gateway_jobs(status, created_at);
CREATE INDEX IF NOT EXISTS idx_gateway_jobs_node ON gateway_jobs(node_id, status);

CREATE TABLE IF NOT EXISTS gateway_job_children (
    parent_job_id UUID NOT NULL REFERENCES gateway_jobs(job_id) ON DELETE CASCADE,
    child_job_id UUID NOT NULL REFERENCES gateway_jobs(job_id) ON DELETE CASCADE,
    child_index INTEGER NOT NULL,
    node_id TEXT,
    account_id TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (parent_job_id, child_job_id),
    UNIQUE (parent_job_id, child_index)
);

CREATE TABLE IF NOT EXISTS job_events (
    event_id BIGSERIAL PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES gateway_jobs(job_id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    data JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_job_events_cursor ON job_events(job_id, event_id);

CREATE TABLE IF NOT EXISTS dispatch_attempts (
    id BIGSERIAL PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES gateway_jobs(job_id) ON DELETE CASCADE,
    node_id TEXT NOT NULL,
    attempt_no INTEGER NOT NULL,
    outcome TEXT NOT NULL,
    http_status INTEGER,
    error TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    UNIQUE (job_id, attempt_no)
);

CREATE TABLE IF NOT EXISTS media_routes (
    media_key TEXT PRIMARY KEY,
    job_id UUID REFERENCES gateway_jobs(job_id) ON DELETE SET NULL,
    node_id TEXT NOT NULL REFERENCES nodes(node_id),
    node_media_id TEXT NOT NULL DEFAULT '',
    account_id TEXT NOT NULL DEFAULT '',
    project_id TEXT NOT NULL DEFAULT '',
    file_name TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    bytes BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS fleet_settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
