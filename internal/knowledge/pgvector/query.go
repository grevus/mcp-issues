package pgvector

import (
	"context"
	"fmt"

	pgvec "github.com/pgvector/pgvector-go"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

const querySQL = `
SELECT doc_key, title, status,
       substring(content, 1, 300) AS excerpt,
       1 - (embedding <=> $1)     AS score
FROM issues_index
WHERE tenant_id = $2
  AND project_key = $3
  AND ($4 = '' OR source = $4)
ORDER BY embedding <=> $1
LIMIT $5`

// Search returns the topK nearest documents to queryEmbedding filtered by f.
// Score is cosine similarity (1 - cosine distance), ranging from 0 to 1.
// If topK is zero or queryEmbedding is empty, an empty slice is returned.
func (s *PgvectorStore) Search(ctx context.Context, queryEmbedding []float32, f knowledge.Filter, topK int) ([]knowledge.Hit, error) {
	if topK == 0 || len(queryEmbedding) == 0 {
		return []knowledge.Hit{}, nil
	}

	rows, err := s.pool.Query(ctx, querySQL,
		pgvec.NewVector(queryEmbedding),
		f.TenantID,
		f.ProjectKey,
		f.Source,
		topK,
	)
	if err != nil {
		return nil, fmt.Errorf("pgvector: Search: %w", err)
	}
	defer rows.Close()

	var hits []knowledge.Hit
	for rows.Next() {
		var h knowledge.Hit
		if err := rows.Scan(&h.DocKey, &h.Title, &h.Status, &h.Excerpt, &h.Score); err != nil {
			return nil, fmt.Errorf("pgvector: Search: scan: %w", err)
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgvector: Search: rows: %w", err)
	}

	if hits == nil {
		hits = []knowledge.Hit{}
	}
	return hits, nil
}
