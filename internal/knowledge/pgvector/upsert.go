package pgvector

import (
	"context"
	"fmt"

	pgvec "github.com/pgvector/pgvector-go"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

const upsertSQL = `
INSERT INTO issues_index (tenant_id, source, project_key, doc_key, title, status, author, content, embedding, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (tenant_id, project_key, doc_key) DO UPDATE SET
    source      = EXCLUDED.source,
    title       = EXCLUDED.title,
    status      = EXCLUDED.status,
    author      = EXCLUDED.author,
    content     = EXCLUDED.content,
    embedding   = EXCLUDED.embedding,
    updated_at  = EXCLUDED.updated_at`

// Upsert inserts or updates the given documents in the store.
// If docs is empty, Upsert returns nil immediately.
func (s *PgvectorStore) Upsert(ctx context.Context, docs []knowledge.Document) error {
	if len(docs) == 0 {
		return nil
	}

	for _, doc := range docs {
		_, err := s.pool.Exec(ctx, upsertSQL,
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
			return fmt.Errorf("pgvector: Upsert: %w", err)
		}
	}

	return nil
}
