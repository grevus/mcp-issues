package store

import (
	"context"
	"fmt"

	pgvector "github.com/pgvector/pgvector-go"
)

const upsertSQL = `
INSERT INTO issues_index (project_key, issue_key, summary, status, assignee, content, embedding, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (issue_key) DO UPDATE SET
    project_key = EXCLUDED.project_key,
    summary     = EXCLUDED.summary,
    status      = EXCLUDED.status,
    assignee    = EXCLUDED.assignee,
    content     = EXCLUDED.content,
    embedding   = EXCLUDED.embedding,
    updated_at  = EXCLUDED.updated_at`

// Upsert inserts or updates the given documents in the store.
// If docs is empty, Upsert returns nil immediately.
func (s *PgvectorStore) Upsert(ctx context.Context, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	for _, doc := range docs {
		_, err := s.pool.Exec(ctx, upsertSQL,
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
			return fmt.Errorf("store: Upsert: %w", err)
		}
	}

	return nil
}
