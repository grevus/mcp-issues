package sqlite

import "database/sql"

const schema = `
CREATE TABLE IF NOT EXISTS issues_index (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id   TEXT NOT NULL DEFAULT '',
    source      TEXT NOT NULL DEFAULT 'jira',
    project_key TEXT NOT NULL,
    doc_key     TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT '',
    author      TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, project_key, doc_key)
);

CREATE INDEX IF NOT EXISTS idx_issues_project ON issues_index (project_key);
CREATE INDEX IF NOT EXISTS idx_issues_tenant_project ON issues_index (tenant_id, project_key);
`

// migrate creates the metadata table and the vec0 virtual table if they don't exist.
func migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// vec0 virtual table for vector search.
	// sqlite-vec does not support IF NOT EXISTS for virtual tables,
	// so we check manually.
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='vec_issues'").Scan(&name)
	if err == sql.ErrNoRows {
		_, err = db.Exec("CREATE VIRTUAL TABLE vec_issues USING vec0(id INTEGER PRIMARY KEY, embedding float[1024])")
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}
