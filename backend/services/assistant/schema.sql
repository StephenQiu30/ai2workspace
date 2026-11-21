-- Schema for assistant service (Knowledge Base, AI Governance, Agent & Search)

CREATE TABLE IF NOT EXISTS knowledge_bases (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    visibility VARCHAR(32) NOT NULL DEFAULT 'private',
    owner_id VARCHAR(64) NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS knowledge_sources (
    id VARCHAR(64) PRIMARY KEY,
    knowledge_base_id VARCHAR(64) NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    kind VARCHAR(32) NOT NULL,
    source_ref VARCHAR(256) NOT NULL,
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    source_version INT NOT NULL DEFAULT 1,
    ready_version INT NOT NULL DEFAULT 0,
    published_version INT NOT NULL DEFAULT 0,
    state VARCHAR(32) NOT NULL DEFAULT 'draft',
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_sources_kb_ref
    ON knowledge_sources (knowledge_base_id, kind, source_ref);

CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id VARCHAR(64) PRIMARY KEY,
    source_id VARCHAR(64) NOT NULL REFERENCES knowledge_sources(id) ON DELETE CASCADE,
    knowledge_base_id VARCHAR(64) NOT NULL,
    course_id VARCHAR(64) NOT NULL DEFAULT '',
    title VARCHAR(256) NOT NULL DEFAULT '',
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    version INT NOT NULL DEFAULT 1,
    is_live BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_source_version
    ON knowledge_chunks (source_id, version, is_live);

CREATE TABLE IF NOT EXISTS knowledge_index_jobs (
    id VARCHAR(64) PRIMARY KEY,
    source_id VARCHAR(64) NOT NULL REFERENCES knowledge_sources(id) ON DELETE CASCADE,
    source_version INT NOT NULL,
    stage VARCHAR(32) NOT NULL,
    state VARCHAR(32) NOT NULL,
    retryable BOOLEAN NOT NULL DEFAULT FALSE,
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    cost_cents INT NOT NULL DEFAULT 0,
    reservation_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_model_catalog (
    provider_id VARCHAR(64) NOT NULL,
    model_id VARCHAR(128) NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    endpoint_ref VARCHAR(256) NOT NULL DEFAULT '',
    price_version VARCHAR(64) NOT NULL DEFAULT 'v1',
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    prompt_price_per_k INT NOT NULL,
    completion_price_per_k INT NOT NULL,
    max_input_tokens INT NOT NULL DEFAULT 8000,
    max_output_tokens INT NOT NULL DEFAULT 2000,
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider_id, model_id)
);

CREATE TABLE IF NOT EXISTS ai_budget_configs (
    id INT PRIMARY KEY DEFAULT 1,
    version INT NOT NULL DEFAULT 1,
    user_daily_questions INT NOT NULL DEFAULT 20,
    user_per_minute INT NOT NULL DEFAULT 3,
    user_concurrency INT NOT NULL DEFAULT 1,
    platform_concurrency INT NOT NULL DEFAULT 10,
    max_input_tokens INT NOT NULL DEFAULT 8000,
    max_output_tokens INT NOT NULL DEFAULT 2000,
    max_single_reserve_cents INT NOT NULL DEFAULT 50,
    user_daily_cents INT NOT NULL DEFAULT 200,
    platform_daily_cents INT NOT NULL DEFAULT 10000,
    platform_monthly_cents INT NOT NULL DEFAULT 200000,
    bg_daily_cents INT NOT NULL DEFAULT 2000,
    alert_threshold_pct INT NOT NULL DEFAULT 80,
    paused BOOLEAN NOT NULL DEFAULT FALSE,
    pause_reason TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_budget_periods (
    scope VARCHAR(32) NOT NULL,
    scope_key VARCHAR(128) NOT NULL,
    period_key VARCHAR(32) NOT NULL,
    confirmed_cents INT NOT NULL DEFAULT 0,
    reserved_cents INT NOT NULL DEFAULT 0,
    success_count INT NOT NULL DEFAULT 0,
    pending_count INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (scope, scope_key, period_key)
);

CREATE TABLE IF NOT EXISTS ai_budget_reservations (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    purpose VARCHAR(32) NOT NULL,
    period_day VARCHAR(16) NOT NULL,
    period_month VARCHAR(16) NOT NULL,
    reserved_cents INT NOT NULL,
    actual_cents INT NOT NULL DEFAULT 0,
    unknown_cents INT NOT NULL DEFAULT 0,
    state VARCHAR(32) NOT NULL,
    reconcile_due_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_budget_ledgers (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    reservation_id VARCHAR(64) NOT NULL,
    step VARCHAR(32) NOT NULL,
    provider VARCHAR(64) NOT NULL,
    model_id VARCHAR(128) NOT NULL,
    prompt_tokens INT NOT NULL DEFAULT 0,
    completion_tokens INT NOT NULL DEFAULT 0,
    cost_cents INT NOT NULL DEFAULT 0,
    price_version VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_budget_reconciliations (
    id VARCHAR(64) PRIMARY KEY,
    reservation_id VARCHAR(64) NOT NULL REFERENCES ai_budget_reservations(id) ON DELETE CASCADE,
    user_id VARCHAR(64) NOT NULL,
    purpose VARCHAR(32) NOT NULL,
    period_day VARCHAR(16) NOT NULL,
    period_month VARCHAR(16) NOT NULL,
    unknown_cents INT NOT NULL,
    resolved_cents INT NOT NULL DEFAULT 0,
    state VARCHAR(32) NOT NULL DEFAULT 'pending',
    reason TEXT NOT NULL DEFAULT '',
    resolution_note TEXT NOT NULL DEFAULT '',
    due_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_configurations (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    slug VARCHAR(64) UNIQUE NOT NULL,
    role_instruction TEXT NOT NULL,
    provider_id VARCHAR(64) NOT NULL,
    model_id VARCHAR(128) NOT NULL,
    knowledge_base_ids JSONB NOT NULL DEFAULT '[]',
    draft_version INT NOT NULL DEFAULT 1,
    published_version INT NOT NULL DEFAULT 0,
    state VARCHAR(32) NOT NULL DEFAULT 'draft',
    disabled_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_sessions (
    id VARCHAR(64) PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL REFERENCES agent_configurations(id) ON DELETE CASCADE,
    user_id VARCHAR(64) NOT NULL,
    course_id VARCHAR(64) NOT NULL DEFAULT '',
    title VARCHAR(256) NOT NULL,
    state VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_runs (
    id VARCHAR(64) PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    agent_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL DEFAULT '',
    question_hash VARCHAR(64) NOT NULL DEFAULT '',
    state VARCHAR(32) NOT NULL,
    phase VARCHAR(32) NOT NULL DEFAULT '',
    message_id VARCHAR(64) NOT NULL DEFAULT '',
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_runs_idempotency
    ON agent_runs (user_id, idempotency_key)
    WHERE idempotency_key <> '';

CREATE TABLE IF NOT EXISTS agent_messages (
    id VARCHAR(64) PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    run_id VARCHAR(64) NOT NULL DEFAULT '',
    sender VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    citations JSONB NOT NULL DEFAULT '[]',
    search_queries JSONB NOT NULL DEFAULT '[]',
    search_status VARCHAR(32) NOT NULL DEFAULT '',
    token_usage JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_search_logs (
    id VARCHAR(64) PRIMARY KEY,
    run_id VARCHAR(64) NOT NULL DEFAULT '',
    message_id VARCHAR(64) NOT NULL DEFAULT '',
    query VARCHAR(512) NOT NULL,
    status VARCHAR(32) NOT NULL,
    attempts INT NOT NULL DEFAULT 1,
    result_count INT NOT NULL DEFAULT 0,
    duration_ms INT NOT NULL DEFAULT 0,
    cost_cents INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
