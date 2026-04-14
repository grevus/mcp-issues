package sqlite

import (
	"context"
	"fmt"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

// Upsert inserts or updates the given documents in the store.
func (s *SqliteStore) Upsert(ctx context.Context, docs []knowledge.Document) error {
	if len(docs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: Upsert: begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, doc := range docs {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO issues_index (tenant_id, source, project_key, doc_key, title, status, author, content, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (tenant_id, project_key, doc_key) DO UPDATE SET
			    source     = excluded.source,
			    title      = excluded.title,
			    status     = excluded.status,
			    author     = excluded.author,
			    content    = excluded.content,
			    updated_at = excluded.updated_at`,
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
			return fmt.Errorf("sqlite: Upsert: insert %s: %w", doc.DocKey, err)
		}

		// Get the rowid for the vec table.
		rowID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("sqlite: Upsert: last insert id %s: %w", doc.DocKey, err)
		}
		// On conflict, LastInsertId may return 0; query the actual id.
		if rowID == 0 {
			err = tx.QueryRowContext(ctx,
				"SELECT id FROM issues_index WHERE tenant_id=? AND project_key=? AND doc_key=?",
				doc.TenantID, doc.ProjectKey, doc.DocKey,
			).Scan(&rowID)
			if err != nil {
				return fmt.Errorf("sqlite: Upsert: get id %s: %w", doc.DocKey, err)
			}
		}

		blob := float32ToBlob(doc.Embedding)

		// Delete old vector if exists, then insert new one.
		if _, err = tx.ExecContext(ctx, "DELETE FROM vec_issues WHERE id = ?", rowID); err != nil {
			return fmt.Errorf("sqlite: Upsert: delete vec %s: %w", doc.DocKey, err)
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO vec_issues (id, embedding) VALUES (?, ?)", rowID, blob); err != nil {
			return fmt.Errorf("sqlite: Upsert: insert vec %s: %w", doc.DocKey, err)
		}
	}

	return tx.Commit()
}
