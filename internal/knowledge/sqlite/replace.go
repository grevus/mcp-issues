package sqlite

import (
	"context"
	"fmt"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

// ReplaceProject atomically removes all documents for tenantID+projectKey
// and inserts docs in a single transaction.
func (s *SqliteStore) ReplaceProject(ctx context.Context, tenantID, projectKey string, docs []knowledge.Document) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: ReplaceProject: begin tx: %w", err)
	}
	defer tx.Rollback()

	// Delete vectors for the project first (need the ids).
	if _, err = tx.ExecContext(ctx,
		"DELETE FROM vec_issues WHERE id IN (SELECT id FROM issues_index WHERE tenant_id=? AND project_key=?)",
		tenantID, projectKey,
	); err != nil {
		return fmt.Errorf("sqlite: ReplaceProject: delete vec: %w", err)
	}

	// Delete metadata rows.
	if _, err = tx.ExecContext(ctx,
		"DELETE FROM issues_index WHERE tenant_id=? AND project_key=?",
		tenantID, projectKey,
	); err != nil {
		return fmt.Errorf("sqlite: ReplaceProject: delete: %w", err)
	}

	// Insert new docs.
	for _, doc := range docs {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO issues_index (tenant_id, source, project_key, doc_key, title, status, author, content, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			doc.TenantID,
			doc.Source,
			doc.ProjectKey,
			doc.DocKey,
			doc.Title,
			doc.Status,
			doc.Author,
			doc.Content,
			doc.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("sqlite: ReplaceProject: insert %s: %w", doc.DocKey, err)
		}

		rowID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("sqlite: ReplaceProject: last insert id %s: %w", doc.DocKey, err)
		}

		blob := float32ToBlob(doc.Embedding)
		if _, err = tx.ExecContext(ctx, "INSERT INTO vec_issues (id, embedding) VALUES (?, ?)", rowID, blob); err != nil {
			return fmt.Errorf("sqlite: ReplaceProject: insert vec %s: %w", doc.DocKey, err)
		}
	}

	return tx.Commit()
}
