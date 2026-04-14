package sqlite

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

// Search returns the topK nearest documents to queryEmbedding filtered by f.
// Score is cosine similarity (1 - distance), ranging from 0 to 1.
func (s *SqliteStore) Search(ctx context.Context, queryEmbedding []float32, f knowledge.Filter, topK int) ([]knowledge.Hit, error) {
	if topK == 0 || len(queryEmbedding) == 0 {
		return []knowledge.Hit{}, nil
	}

	blob := float32ToBlob(queryEmbedding)

	// sqlite-vec vec0 KNN requires a `k = ?` constraint directly on the virtual
	// table. We over-fetch candidates and apply tenant/project filters via JOIN,
	// then let SQL LIMIT cut to topK. Over-fetch factor protects correctness in
	// multi-tenant stores where many nearest neighbors may belong to other tenants.
	k := max(topK*10, 50)

	rows, err := s.db.QueryContext(ctx, `
		WITH matches AS (
		    SELECT id, distance
		    FROM vec_issues
		    WHERE embedding MATCH ? AND k = ?
		)
		SELECT i.doc_key, i.title, i.status,
		       substr(i.content, 1, 300) AS excerpt,
		       (1 - m.distance) AS score
		FROM matches m
		JOIN issues_index i ON i.id = m.id
		WHERE i.tenant_id = ?
		  AND i.project_key = ?
		  AND (? = '' OR i.source = ?)
		ORDER BY m.distance
		LIMIT ?`,
		blob, k,
		f.TenantID,
		f.ProjectKey,
		f.Source, f.Source,
		topK,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: Search: %w", err)
	}
	defer rows.Close()

	var hits []knowledge.Hit
	for rows.Next() {
		var h knowledge.Hit
		if err := rows.Scan(&h.DocKey, &h.Title, &h.Status, &h.Excerpt, &h.Score); err != nil {
			return nil, fmt.Errorf("sqlite: Search: scan: %w", err)
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: Search: rows: %w", err)
	}

	if hits == nil {
		hits = []knowledge.Hit{}
	}
	return hits, nil
}

// float32ToBlob converts a float32 slice to a little-endian byte blob
// as expected by sqlite-vec.
func float32ToBlob(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}
