-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE issues_index (
    id          bigserial PRIMARY KEY,
    project_key text        NOT NULL,
    issue_key   text        NOT NULL UNIQUE,
    summary     text        NOT NULL DEFAULT '',
    status      text        NOT NULL DEFAULT '',
    assignee    text        NOT NULL DEFAULT '',
    content     text        NOT NULL DEFAULT '',
    embedding   vector(1024),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON issues_index USING hnsw (embedding vector_cosine_ops);
CREATE INDEX ON issues_index (project_key);

-- +goose Down
DROP TABLE IF EXISTS issues_index;
DROP EXTENSION IF EXISTS vector;
