-- Schema for commerce service

CREATE TABLE IF NOT EXISTS prices (
    course_id VARCHAR(64) PRIMARY KEY,
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    version INT NOT NULL DEFAULT 1,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
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

CREATE INDEX IF NOT EXISTS idx_orders_user_created ON orders(user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_orders_course_id ON orders(course_id);

CREATE TABLE IF NOT EXISTS payment_attempts (
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

CREATE INDEX IF NOT EXISTS idx_payments_order_id ON payment_attempts(order_id, created_at DESC);

CREATE TABLE IF NOT EXISTS payment_anomalies (
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

CREATE INDEX IF NOT EXISTS idx_payment_anomalies_order_id ON payment_anomalies(order_id);

CREATE TABLE IF NOT EXISTS entitlements (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    course_id VARCHAR(64) NOT NULL,
    order_id VARCHAR(64) NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    grant_version INT NOT NULL DEFAULT 1,
    activated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_entitlements_user_course ON entitlements(user_id, course_id, status);
CREATE INDEX IF NOT EXISTS idx_entitlements_user_activated ON entitlements(user_id, activated_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS refund_applications (
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

CREATE INDEX IF NOT EXISTS idx_refunds_order_id ON refund_applications(order_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_refunds_user_id ON refund_applications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_refunds_created_at ON refund_applications(created_at DESC, id DESC);
