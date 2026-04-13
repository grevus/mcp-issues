package pgvector

import (
	"context"
	"fmt"

	pgvec "github.com/pgvector/pgvector-go"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

const deleteProjectSQL = `DELETE FROM issues_index WHERE tenant_id = $1 AND project_key = $2`

// ReplaceProject atomically removes all documents for tenantID+projectKey and inserts
// docs in a single transaction. On any error the transaction is rolled back.
func (s *PgvectorStore) ReplaceProject(ctx context.Context, tenantID, projectKey string, docs []knowledge.Document) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pgvector: ReplaceProject: begin tx: %w", err)
	}
	defer func() {
		// Rollback is a no-op when the transaction has already been committed.
		_ = tx.Rollback(ctx)
	}()

	if _, err = tx.Exec(ctx, deleteProjectSQL, tenantID, projectKey); err != nil {
		return fmt.Errorf("pgvector: ReplaceProject: delete: %w", err)
	}

	for _, doc := range docs {
		_, err = tx.Exec(ctx, upsertSQL,
			doc.TenantID,
			doc.Source,
			doc.ProjectKey,
			doc.DocKey,
			doc.Title,
			doc.Status,
			doc.Author,
			doc.Content,
			pgvec.NewVector(doc.Embedding),
			doc.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("pgvector: ReplaceProject: insert %s: %w", doc.DocKey, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("pgvector: ReplaceProject: commit: %w", err)
	}

	return nil
}
