-- +goose Up
ALTER TABLE issues_index ADD COLUMN tenant_id TEXT NOT NULL DEFAULT '';
ALTER TABLE issues_index ADD COLUMN source TEXT NOT NULL DEFAULT 'jira';
ALTER TABLE issues_index RENAME COLUMN issue_key TO doc_key;
ALTER TABLE issues_index RENAME COLUMN summary TO title;
ALTER TABLE issues_index RENAME COLUMN assignee TO author;

ALTER TABLE issues_index DROP CONSTRAINT IF EXISTS issues_index_issue_key_key;
ALTER TABLE issues_index ADD CONSTRAINT issues_index_tenant_project_doc_key
    UNIQUE (tenant_id, project_key, doc_key);

-- +goose Down
ALTER TABLE issues_index DROP CONSTRAINT IF EXISTS issues_index_tenant_project_doc_key;
ALTER TABLE issues_index RENAME COLUMN doc_key TO issue_key;
ALTER TABLE issues_index RENAME COLUMN title TO summary;
ALTER TABLE issues_index RENAME COLUMN author TO assignee;
ALTER TABLE issues_index DROP COLUMN IF EXISTS source;
ALTER TABLE issues_index DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE issues_index ADD CONSTRAINT issues_index_issue_key_key UNIQUE (issue_key);
