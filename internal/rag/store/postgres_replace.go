package store

import (
	"context"
	"fmt"

	pgvector "github.com/pgvector/pgvector-go"
)

const deleteProjectSQL = `DELETE FROM issues_index WHERE project_key = $1`

// ReplaceProject atomically removes all documents for projectKey and inserts
// docs in a single transaction. On any error the transaction is rolled back.
func (s *PgvectorStore) ReplaceProject(ctx context.Context, projectKey string, docs []Document) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: ReplaceProject: begin tx: %w", err)
	}
	defer func() {
		// Rollback is a no-op when the transaction has already been committed.
		_ = tx.Rollback(ctx)
	}()

	if _, err = tx.Exec(ctx, deleteProjectSQL, projectKey); err != nil {
		return fmt.Errorf("store: ReplaceProject: delete: %w", err)
	}

	for _, doc := range docs {
		_, err = tx.Exec(ctx, upsertSQL,
			doc.ProjectKey,
			doc.IssueKey,
			doc.Summary,
			doc.Status,
			doc.Assignee,
			doc.Content,
			pgvector.NewVector(doc.Embedding),
			doc.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("store: ReplaceProject: insert %s: %w", doc.IssueKey, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: ReplaceProject: commit: %w", err)
	}

	return nil
}
