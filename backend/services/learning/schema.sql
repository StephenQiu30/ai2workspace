-- Schema for learning service

CREATE TABLE IF NOT EXISTS courses (
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

ALTER TABLE courses ADD COLUMN IF NOT EXISTS author_nickname VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS category VARCHAR(64) NOT NULL DEFAULT 'general';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS review_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_courses_owner_id ON courses(owner_id);
CREATE INDEX IF NOT EXISTS idx_courses_production_state ON courses(production_state);
CREATE INDEX IF NOT EXISTS idx_courses_category ON courses(category);
CREATE INDEX IF NOT EXISTS idx_courses_updated_at ON courses(updated_at DESC);

CREATE TABLE IF NOT EXISTS sections (
    id VARCHAR(64) PRIMARY KEY,
    course_id VARCHAR(64) NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sections_course_id ON sections(course_id);

CREATE TABLE IF NOT EXISTS lessons (
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

CREATE INDEX IF NOT EXISTS idx_lessons_course_id ON lessons(course_id);
CREATE INDEX IF NOT EXISTS idx_lessons_section_id ON lessons(section_id);

CREATE TABLE IF NOT EXISTS enrollments (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    course_id VARCHAR(64) NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    type VARCHAR(32) NOT NULL DEFAULT 'free',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, course_id)
);

CREATE INDEX IF NOT EXISTS idx_enrollments_user_id ON enrollments(user_id);

CREATE TABLE IF NOT EXISTS uploads (
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

ALTER TABLE uploads ADD COLUMN IF NOT EXISTS upload_instructions JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE uploads ADD COLUMN IF NOT EXISTS media_version_id VARCHAR(64) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_uploads_instructor_id ON uploads(instructor_id);
CREATE INDEX IF NOT EXISTS idx_uploads_lesson_id ON uploads(lesson_id);

CREATE TABLE IF NOT EXISTS media_versions (
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

ALTER TABLE media_versions ADD COLUMN IF NOT EXISTS hls_qualities TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_media_versions_lesson_id ON media_versions(lesson_id);

CREATE TABLE IF NOT EXISTS upload_idempotency (
    upload_id VARCHAR(64) NOT NULL REFERENCES uploads(id) ON DELETE CASCADE,
    idempotency_key VARCHAR(128) NOT NULL,
    payload_hash VARCHAR(64) NOT NULL,
    media_version_id VARCHAR(64) NOT NULL REFERENCES media_versions(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (upload_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS lesson_media (
    lesson_id VARCHAR(64) PRIMARY KEY REFERENCES lessons(id) ON DELETE CASCADE,
    media_version_id VARCHAR(64) NOT NULL REFERENCES media_versions(id) ON DELETE CASCADE,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS media_jobs (
    id VARCHAR(64) PRIMARY KEY,
    upload_id VARCHAR(64) NOT NULL REFERENCES uploads(id) ON DELETE CASCADE,
    media_version_id VARCHAR(64) NOT NULL REFERENCES media_versions(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    retry_count INT NOT NULL DEFAULT 0,
    error_message VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS learning_progress (
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

CREATE INDEX IF NOT EXISTS idx_progress_user_id ON learning_progress(user_id);
CREATE INDEX IF NOT EXISTS idx_progress_user_course ON learning_progress(user_id, course_id);
