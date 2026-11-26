-- Canonical PostgreSQL schema for a new ai2workspace database.
BEGIN;

-- identity
CREATE TABLE accounts (
    id VARCHAR(64) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    nickname VARCHAR(64) NOT NULL DEFAULT '',
    password_hash VARCHAR(255) NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    roles TEXT[] NOT NULL DEFAULT '{"student"}',
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_email ON accounts(email);
CREATE INDEX idx_accounts_created_at ON accounts(created_at DESC);

CREATE TABLE sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);

CREATE TABLE one_time_tokens (
    token_hash VARCHAR(64) PRIMARY KEY,
    purpose VARCHAR(64) NOT NULL,
    target VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    consumed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_one_time_tokens_target ON one_time_tokens(target);

CREATE TABLE instructor_invitations (
    id VARCHAR(64) PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    invited_by VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    token_hash VARCHAR(64) NOT NULL DEFAULT '',
    version INT NOT NULL DEFAULT 1,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT (NOW() + INTERVAL '7 days'),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_instructor_invitations_email ON instructor_invitations(email);
CREATE UNIQUE INDEX idx_instructor_invitations_token_hash ON instructor_invitations(token_hash) WHERE token_hash <> '';
CREATE INDEX idx_instructor_invitations_created_at ON instructor_invitations(created_at DESC);

CREATE TABLE mail_tasks (
    id VARCHAR(64) PRIMARY KEY,
    recipient VARCHAR(255) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    purpose VARCHAR(64) NOT NULL,
    token VARCHAR(255),
    status VARCHAR(32) NOT NULL DEFAULT 'sent',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mail_tasks_recipient ON mail_tasks(recipient, created_at DESC);

CREATE TABLE audit_logs (
    id VARCHAR(64) PRIMARY KEY,
    actor_id VARCHAR(64) NOT NULL,
    target_id VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_target_id ON audit_logs(target_id, created_at DESC);
CREATE INDEX idx_audit_logs_actor_id ON audit_logs(actor_id, created_at DESC);

-- learning
CREATE TABLE courses (
    id VARCHAR(64) PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL,
    author_nickname VARCHAR(128) NOT NULL DEFAULT '',
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    cover_url VARCHAR(512) NOT NULL DEFAULT '',
    category VARCHAR(64) NOT NULL DEFAULT 'general',
    is_free BOOLEAN NOT NULL DEFAULT FALSE,
    production_state VARCHAR(32) NOT NULL DEFAULT 'draft',
    sale_state VARCHAR(32) NOT NULL DEFAULT 'for_sale',
    delivery_state VARCHAR(32) NOT NULL DEFAULT 'available',
    review_reason TEXT NOT NULL DEFAULT '',
    published_version INT NOT NULL DEFAULT 0,
    version INT NOT NULL DEFAULT 1,
    first_deliverable_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);


CREATE INDEX idx_courses_owner_id ON courses(owner_id);
CREATE INDEX idx_courses_production_state ON courses(production_state);
CREATE INDEX idx_courses_category ON courses(category);
CREATE INDEX idx_courses_updated_at ON courses(updated_at DESC);

CREATE TABLE sections (
    id VARCHAR(64) PRIMARY KEY,
    course_id VARCHAR(64) NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sections_course_id ON sections(course_id);

CREATE TABLE lessons (
    id VARCHAR(64) PRIMARY KEY,
    section_id VARCHAR(64) NOT NULL REFERENCES sections(id) ON DELETE CASCADE,
    course_id VARCHAR(64) NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'video',
    is_trial BOOLEAN NOT NULL DEFAULT FALSE,
    content TEXT NOT NULL DEFAULT '',
    media_version_id VARCHAR(64) NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lessons_course_id ON lessons(course_id);
CREATE INDEX idx_lessons_section_id ON lessons(section_id);

CREATE TABLE enrollments (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    course_id VARCHAR(64) NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    type VARCHAR(32) NOT NULL DEFAULT 'free',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, course_id)
);

CREATE INDEX idx_enrollments_user_id ON enrollments(user_id);

CREATE TABLE uploads (
    id VARCHAR(64) PRIMARY KEY,
    instructor_id VARCHAR(64) NOT NULL,
    course_id VARCHAR(64) NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    lesson_id VARCHAR(64) NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    declared_bytes BIGINT NOT NULL,
    actual_bytes BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'uploading',
    source_version INT NOT NULL DEFAULT 1,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    upload_instructions JSONB NOT NULL DEFAULT '{}'::jsonb,
    media_version_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);


CREATE INDEX idx_uploads_instructor_id ON uploads(instructor_id);
CREATE INDEX idx_uploads_lesson_id ON uploads(lesson_id);

CREATE TABLE media_versions (
    id VARCHAR(64) PRIMARY KEY,
    upload_id VARCHAR(64) REFERENCES uploads(id) ON DELETE SET NULL,
    lesson_id VARCHAR(64) NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'processing',
    duration_seconds INT NOT NULL DEFAULT 0,
    width INT NOT NULL DEFAULT 0,
    height INT NOT NULL DEFAULT 0,
    fps INT NOT NULL DEFAULT 0,
    format VARCHAR(32) NOT NULL DEFAULT 'hls',
    playback_url VARCHAR(512) NOT NULL DEFAULT '',
    hls_qualities TEXT[] NOT NULL DEFAULT '{}',
    failure_reason VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);


CREATE INDEX idx_media_versions_lesson_id ON media_versions(lesson_id);

CREATE TABLE upload_idempotency (
    upload_id VARCHAR(64) NOT NULL REFERENCES uploads(id) ON DELETE CASCADE,
    idempotency_key VARCHAR(128) NOT NULL,
    payload_hash VARCHAR(64) NOT NULL,
    media_version_id VARCHAR(64) NOT NULL REFERENCES media_versions(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (upload_id, idempotency_key)
);

CREATE TABLE lesson_media (
    lesson_id VARCHAR(64) PRIMARY KEY REFERENCES lessons(id) ON DELETE CASCADE,
    media_version_id VARCHAR(64) NOT NULL REFERENCES media_versions(id) ON DELETE CASCADE,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE media_jobs (
    id VARCHAR(64) PRIMARY KEY,
    upload_id VARCHAR(64) NOT NULL REFERENCES uploads(id) ON DELETE CASCADE,
    media_version_id VARCHAR(64) NOT NULL REFERENCES media_versions(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    retry_count INT NOT NULL DEFAULT 0,
    error_message VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE learning_progress (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    lesson_id VARCHAR(64) NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    course_id VARCHAR(64) NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    content_version INT NOT NULL DEFAULT 1,
    position_seconds INT NOT NULL DEFAULT 0,
    version INT NOT NULL DEFAULT 1,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, lesson_id)
);

CREATE INDEX idx_progress_user_id ON learning_progress(user_id);
CREATE INDEX idx_progress_user_course ON learning_progress(user_id, course_id);

-- commerce
CREATE TABLE prices (
    course_id VARCHAR(64) PRIMARY KEY,
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    version INT NOT NULL DEFAULT 1,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE orders (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    course_id VARCHAR(64) NOT NULL,
    course_title VARCHAR(255) NOT NULL DEFAULT '',
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    status VARCHAR(32) NOT NULL DEFAULT 'pending_payment',
    idempotency_key VARCHAR(128) NOT NULL,
    policy_version VARCHAR(32) NOT NULL DEFAULT 'v1',
    paid_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, idempotency_key)
);

CREATE INDEX idx_orders_user_created ON orders(user_id, created_at DESC, id DESC);
CREATE INDEX idx_orders_course_id ON orders(course_id);

CREATE TABLE payment_attempts (
    id VARCHAR(64) PRIMARY KEY,
    order_id VARCHAR(64) NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    channel VARCHAR(32) NOT NULL DEFAULT 'mock',
    channel_order_id VARCHAR(128) NOT NULL DEFAULT '',
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_order_id ON payment_attempts(order_id, created_at DESC);

CREATE TABLE payment_anomalies (
    id VARCHAR(64) PRIMARY KEY,
    order_id VARCHAR(64) NOT NULL,
    channel VARCHAR(32) NOT NULL,
    channel_order_id VARCHAR(128) NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency VARCHAR(16) NOT NULL,
    order_status VARCHAR(32) NOT NULL,
    reason VARCHAR(128) NOT NULL,
    recorded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_anomalies_order_id ON payment_anomalies(order_id);

CREATE TABLE entitlements (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    course_id VARCHAR(64) NOT NULL,
    order_id VARCHAR(64) NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    grant_version INT NOT NULL DEFAULT 1,
    activated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_entitlements_user_course ON entitlements(user_id, course_id, status);
CREATE INDEX idx_entitlements_user_activated ON entitlements(user_id, activated_at DESC, id DESC);

CREATE TABLE refund_applications (
    id VARCHAR(64) PRIMARY KEY,
    order_id VARCHAR(64) NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    user_id VARCHAR(64) NOT NULL,
    reason VARCHAR(500) NOT NULL,
    exception_type VARCHAR(64) NOT NULL DEFAULT '',
    is_out_of_window BOOLEAN NOT NULL DEFAULT FALSE,
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    application_state VARCHAR(32) NOT NULL DEFAULT 'pending_review',
    channel_state VARCHAR(32) NOT NULL DEFAULT 'none',
    channel_refund_id VARCHAR(128) NOT NULL DEFAULT '',
    channel_failure_reason VARCHAR(255) NOT NULL DEFAULT '',
    reviewer_id VARCHAR(64) NOT NULL DEFAULT '',
    reviewed_at TIMESTAMP WITH TIME ZONE,
    rejection_reason VARCHAR(500) NOT NULL DEFAULT '',
    version INT NOT NULL DEFAULT 1,
    first_available_at TIMESTAMP WITH TIME ZONE,
    deadline TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refunds_order_id ON refund_applications(order_id, created_at DESC);
CREATE INDEX idx_refunds_user_id ON refund_applications(user_id, created_at DESC);
CREATE INDEX idx_refunds_created_at ON refund_applications(created_at DESC, id DESC);

-- community
CREATE TABLE posts (
    id VARCHAR(64) PRIMARY KEY,
    scope VARCHAR(32) NOT NULL DEFAULT 'global',
    course_id VARCHAR(64) NOT NULL DEFAULT '',
    topic VARCHAR(64) NOT NULL DEFAULT 'general',
    author_id VARCHAR(64) NOT NULL,
    author_nickname VARCHAR(128) NOT NULL DEFAULT '',
    title VARCHAR(255) NOT NULL,
    public_summary TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'published',
    version INT NOT NULL DEFAULT 1,
    request_id VARCHAR(128),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_posts_author ON posts(author_id);
CREATE INDEX idx_posts_scope_course_created ON posts(scope, course_id, created_at DESC, id DESC);
CREATE INDEX idx_posts_topic_status_created ON posts(topic, status, created_at DESC, id DESC);
CREATE UNIQUE INDEX idx_posts_author_request_id ON posts(author_id, request_id) WHERE request_id IS NOT NULL AND request_id <> '';

CREATE TABLE comments (
    id VARCHAR(64) PRIMARY KEY,
    post_id VARCHAR(64) NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    author_id VARCHAR(64) NOT NULL,
    author_nickname VARCHAR(128) NOT NULL DEFAULT '',
    parent_comment_id VARCHAR(64) REFERENCES comments(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'published',
    version INT NOT NULL DEFAULT 1,
    request_id VARCHAR(128),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_comments_post_created ON comments(post_id, created_at ASC, id ASC);
CREATE UNIQUE INDEX idx_comments_author_request_id ON comments(author_id, request_id) WHERE request_id IS NOT NULL AND request_id <> '';

CREATE TABLE reactions (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    target_type VARCHAR(32) NOT NULL,
    target_id VARCHAR(64) NOT NULL,
    value VARCHAR(16) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, target_type, target_id)
);

CREATE INDEX idx_reactions_target ON reactions(target_type, target_id);

CREATE TABLE favorites (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    post_id VARCHAR(64) NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, post_id)
);

CREATE INDEX idx_favorites_user_created ON favorites(user_id, created_at DESC, post_id DESC);

CREATE TABLE reports (
    id VARCHAR(64) PRIMARY KEY,
    reporter_id VARCHAR(64) NOT NULL,
    target_type VARCHAR(32) NOT NULL,
    target_id VARCHAR(64) NOT NULL,
    reason VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    review_reason VARCHAR(255) NOT NULL DEFAULT '',
    reviewer_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_reports_status_created ON reports(status, created_at DESC, id DESC);
CREATE INDEX idx_reports_pending_dedup ON reports(reporter_id, target_type, target_id) WHERE status = 'pending';

CREATE TABLE notifications (
    id VARCHAR(64) PRIMARY KEY,
    event_id VARCHAR(128) NOT NULL,
    recipient_id VARCHAR(64) NOT NULL,
    type VARCHAR(32) NOT NULL,
    title VARCHAR(255) NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    target_type VARCHAR(32) NOT NULL DEFAULT '',
    target_id VARCHAR(64) NOT NULL DEFAULT '',
    read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(event_id, recipient_id)
);

CREATE INDEX idx_notifications_recipient_created ON notifications(recipient_id, created_at DESC, id DESC);

-- assistant
CREATE TABLE knowledge_bases (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    visibility VARCHAR(32) NOT NULL DEFAULT 'private',
    owner_id VARCHAR(64) NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE knowledge_sources (
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

CREATE INDEX idx_knowledge_sources_kb_ref
    ON knowledge_sources (knowledge_base_id, kind, source_ref);

CREATE TABLE knowledge_chunks (
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

CREATE INDEX idx_knowledge_chunks_source_version
    ON knowledge_chunks (source_id, version, is_live);

CREATE TABLE knowledge_index_jobs (
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

CREATE TABLE ai_model_catalog (
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

CREATE TABLE ai_budget_configs (
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

CREATE TABLE ai_budget_periods (
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

CREATE TABLE ai_budget_reservations (
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

CREATE TABLE ai_budget_ledgers (
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

CREATE TABLE ai_budget_reconciliations (
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

CREATE TABLE agent_configurations (
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

CREATE TABLE agent_sessions (
    id VARCHAR(64) PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL REFERENCES agent_configurations(id) ON DELETE CASCADE,
    user_id VARCHAR(64) NOT NULL,
    course_id VARCHAR(64) NOT NULL DEFAULT '',
    title VARCHAR(256) NOT NULL,
    state VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE agent_runs (
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

CREATE INDEX idx_agent_runs_idempotency
    ON agent_runs (user_id, idempotency_key)
    WHERE idempotency_key <> '';

CREATE TABLE agent_messages (
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

CREATE TABLE agent_search_logs (
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

-- studio
CREATE TABLE workspaces (
    id text PRIMARY KEY
);

CREATE TABLE workspace_members (
    workspace_id text NOT NULL REFERENCES workspaces (id),
    user_id varchar(64) NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('owner', 'member')),
    revoked_at timestamptz,
    PRIMARY KEY (workspace_id, user_id)
);

COMMIT;
