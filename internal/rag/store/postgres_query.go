package store

import (
	"context"
	"fmt"

	pgvector "github.com/pgvector/pgvector-go"
)

const querySQL = `
SELECT issue_key, summary, status,
       substring(content, 1, 300) AS excerpt,
       1 - (embedding <=> $1)     AS score
FROM issues_index
WHERE project_key = $2
ORDER BY embedding <=> $1
LIMIT $3`

// Query returns the topK nearest documents to queryEmbedding filtered by f.
// Score is cosine similarity (1 - cosine distance), ranging from 0 to 1.
// If topK is zero or queryEmbedding is empty, an empty slice is returned.
func (s *PgvectorStore) Query(ctx context.Context, queryEmbedding []float32, f Filter, topK int) ([]Hit, error) {
	if topK == 0 || len(queryEmbedding) == 0 {
		return []Hit{}, nil
	}

	rows, err := s.pool.Query(ctx, querySQL,
		pgvector.NewVector(queryEmbedding),
		f.ProjectKey,
		topK,
	)
	if err != nil {
		return nil, fmt.Errorf("store: Query: %w", err)
	}
	defer rows.Close()

	var hits []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.IssueKey, &h.Summary, &h.Status, &h.Excerpt, &h.Score); err != nil {
			return nil, fmt.Errorf("store: Query: scan: %w", err)
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: Query: rows: %w", err)
	}

	if hits == nil {
		hits = []Hit{}
	}
	return hits, nil
}
