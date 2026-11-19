-- Schema for commerce service

CREATE TABLE IF NOT EXISTS prices (
    course_id VARCHAR(64) PRIMARY KEY,
    amount_minor BIGINT NOT NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    version INT NOT NULL DEFAULT 1,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    course_id VARCHAR(64) NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    policy_version VARCHAR(32) NOT NULL DEFAULT 'v1',
    paid_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_course_id ON orders(course_id);

CREATE TABLE IF NOT EXISTS payment_attempts (
    id VARCHAR(64) PRIMARY KEY,
    order_id VARCHAR(64) NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    channel VARCHAR(32) NOT NULL DEFAULT 'mock',
    channel_order_id VARCHAR(128),
    amount_minor BIGINT NOT NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_order_id ON payment_attempts(order_id);

CREATE TABLE IF NOT EXISTS entitlements (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    course_id VARCHAR(64) NOT NULL,
    order_id VARCHAR(64) NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    grant_version INT NOT NULL DEFAULT 1,
    activated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(user_id, course_id, order_id)
);

CREATE INDEX IF NOT EXISTS idx_entitlements_user_course ON entitlements(user_id, course_id);

CREATE TABLE IF NOT EXISTS refund_applications (
    id VARCHAR(64) PRIMARY KEY,
    order_id VARCHAR(64) NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    user_id VARCHAR(64) NOT NULL,
    reason VARCHAR(255) NOT NULL,
    amount_minor BIGINT NOT NULL,
    application_state VARCHAR(32) NOT NULL DEFAULT 'pending',
    channel_state VARCHAR(32) NOT NULL DEFAULT 'none',
    rejection_reason VARCHAR(255) NOT NULL DEFAULT '',
    version INT NOT NULL DEFAULT 1,
    deadline TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refunds_order_id ON refund_applications(order_id);
CREATE INDEX IF NOT EXISTS idx_refunds_user_id ON refund_applications(user_id);
