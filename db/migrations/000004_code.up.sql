-- Per-tenant code execution provider configuration (e.g. a private Judge0 URL + auth token).
CREATE TABLE code_provider_configs (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider         TEXT        NOT NULL,
    encrypted_config BYTEA       NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, provider)
);

-- Stores the output of async code execution jobs so callers can retrieve results
-- after polling GET /jobs/:id to completion.
CREATE TABLE code_executions (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id         UUID        NOT NULL REFERENCES jobs(id),
    tenant_id      UUID        NOT NULL REFERENCES tenants(id),
    stdout         TEXT        NOT NULL DEFAULT '',
    stderr         TEXT        NOT NULL DEFAULT '',
    compile_output TEXT        NOT NULL DEFAULT '',
    status         TEXT        NOT NULL DEFAULT '',
    exec_time      TEXT        NOT NULL DEFAULT '',
    memory         INT         NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
