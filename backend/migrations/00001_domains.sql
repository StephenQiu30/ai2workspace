-- +goose Up
CREATE SCHEMA identity;

CREATE TABLE identity.workspaces (
    id text PRIMARY KEY
);

CREATE TABLE identity.workspace_members (
    workspace_id text NOT NULL REFERENCES identity.workspaces (id),
    user_id text NOT NULL,
    role text NOT NULL CHECK (role IN ('owner', 'member')),
    revoked_at timestamptz,
    PRIMARY KEY (workspace_id, user_id)
);

-- +goose Down
DROP TABLE identity.workspace_members;
DROP TABLE identity.workspaces;
DROP SCHEMA identity;
