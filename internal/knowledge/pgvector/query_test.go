//go:build integration

package pgvector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

// unit vector of length 1024 with a 1.0 at position pos.
func unitVec(pos int) []float32 {
	v := make([]float32, 1024)
	v[pos] = 1.0
	return v
}

func TestPgvectorStore_Query(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()

	docs := []knowledge.Document{
		{
			TenantID:   "",
			Source:     "jira",
			ProjectKey: "ALPHA",
			DocKey:     "ALPHA-1",
			Title:      "First issue",
			Status:     "Open",
			Author:     "alice",
			Content:    "Content of the first issue, should be returned as top hit.",
			Embedding:  unitVec(0),
			UpdatedAt:  time.Now().UTC(),
		},
		{
			TenantID:   "",
			Source:     "jira",
			ProjectKey: "ALPHA",
			DocKey:     "ALPHA-2",
			Title:      "Second issue",
			Status:     "In Progress",
			Author:     "bob",
			Content:    "Content of the second issue.",
			Embedding:  unitVec(1),
			UpdatedAt:  time.Now().UTC(),
		},
		{
			TenantID:   "",
			Source:     "jira",
			ProjectKey: "ALPHA",
			DocKey:     "ALPHA-3",
			Title:      "Third issue",
			Status:     "Done",
			Author:     "carol",
			Content:    "Content of the third issue.",
			Embedding:  unitVec(2),
			UpdatedAt:  time.Now().UTC(),
		},
	}

	err := st.Upsert(ctx, docs)
	require.NoError(t, err)

	// Query embedding is identical to ALPHA-1 embedding → should be top hit.
	queryEmb := unitVec(0)

	hits, err := st.Search(ctx, queryEmb, knowledge.Filter{TenantID: "", ProjectKey: "ALPHA"}, 3)
	require.NoError(t, err)
	require.Len(t, hits, 3, "expected 3 hits for topK=3")
	require.Equal(t, "ALPHA-1", hits[0].DocKey, "ALPHA-1 must be the top hit (closest to query embedding)")
	require.NotEmpty(t, hits[0].Excerpt, "excerpt must not be empty")
}

func TestPgvectorStore_Query_EmptyWhenNoMatch(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()

	hits, err := st.Search(ctx, unitVec(0), knowledge.Filter{TenantID: "", ProjectKey: "NOSUCHPROJECT"}, 5)
	require.NoError(t, err)
	require.Empty(t, hits)
}

func TestPgvectorStore_Query_ZeroTopK(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()

	hits, err := st.Search(ctx, unitVec(0), knowledge.Filter{TenantID: "", ProjectKey: "ALPHA"}, 0)
	require.NoError(t, err)
	require.Empty(t, hits)
}
