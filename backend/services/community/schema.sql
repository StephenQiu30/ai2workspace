-- Schema for community service

CREATE TABLE IF NOT EXISTS posts (
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

CREATE INDEX IF NOT EXISTS idx_posts_author ON posts(author_id);
CREATE INDEX IF NOT EXISTS idx_posts_scope_course_created ON posts(scope, course_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_posts_topic_status_created ON posts(topic, status, created_at DESC, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_posts_author_request_id ON posts(author_id, request_id) WHERE request_id IS NOT NULL AND request_id <> '';

CREATE TABLE IF NOT EXISTS comments (
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

CREATE INDEX IF NOT EXISTS idx_comments_post_created ON comments(post_id, created_at ASC, id ASC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_comments_author_request_id ON comments(author_id, request_id) WHERE request_id IS NOT NULL AND request_id <> '';

CREATE TABLE IF NOT EXISTS reactions (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    target_type VARCHAR(32) NOT NULL,
    target_id VARCHAR(64) NOT NULL,
    value VARCHAR(16) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, target_type, target_id)
);

CREATE INDEX IF NOT EXISTS idx_reactions_target ON reactions(target_type, target_id);

CREATE TABLE IF NOT EXISTS favorites (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    post_id VARCHAR(64) NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, post_id)
);

CREATE INDEX IF NOT EXISTS idx_favorites_user_created ON favorites(user_id, created_at DESC, post_id DESC);

CREATE TABLE IF NOT EXISTS reports (
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

CREATE INDEX IF NOT EXISTS idx_reports_status_created ON reports(status, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_reports_pending_dedup ON reports(reporter_id, target_type, target_id) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS notifications (
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

CREATE INDEX IF NOT EXISTS idx_notifications_recipient_created ON notifications(recipient_id, created_at DESC, id DESC);
