-- Schema for identity service

CREATE TABLE IF NOT EXISTS accounts (
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

ALTER TABLE accounts ADD COLUMN IF NOT EXISTS nickname VARCHAR(64) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_accounts_email ON accounts(email);
CREATE INDEX IF NOT EXISTS idx_accounts_created_at ON accounts(created_at DESC);

CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);

CREATE TABLE IF NOT EXISTS one_time_tokens (
    token_hash VARCHAR(64) PRIMARY KEY,
    purpose VARCHAR(64) NOT NULL,
    target VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    consumed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_one_time_tokens_target ON one_time_tokens(target);

CREATE TABLE IF NOT EXISTS instructor_invitations (
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

ALTER TABLE instructor_invitations ADD COLUMN IF NOT EXISTS token_hash VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE instructor_invitations ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT (NOW() + INTERVAL '7 days');
CREATE INDEX IF NOT EXISTS idx_instructor_invitations_email ON instructor_invitations(email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_instructor_invitations_token_hash ON instructor_invitations(token_hash) WHERE token_hash <> '';
CREATE INDEX IF NOT EXISTS idx_instructor_invitations_created_at ON instructor_invitations(created_at DESC);

CREATE TABLE IF NOT EXISTS mail_tasks (
    id VARCHAR(64) PRIMARY KEY,
    recipient VARCHAR(255) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    purpose VARCHAR(64) NOT NULL,
    token VARCHAR(255),
    status VARCHAR(32) NOT NULL DEFAULT 'sent',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mail_tasks_recipient ON mail_tasks(recipient, created_at DESC);

CREATE TABLE IF NOT EXISTS audit_logs (
    id VARCHAR(64) PRIMARY KEY,
    actor_id VARCHAR(64) NOT NULL,
    target_id VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_target_id ON audit_logs(target_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs(actor_id, created_at DESC);
