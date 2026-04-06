//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

	docs := []Document{
		{
			ProjectKey: "ALPHA",
			IssueKey:   "ALPHA-1",
			Summary:    "First issue",
			Status:     "Open",
			Assignee:   "alice",
			Content:    "Content of the first issue, should be returned as top hit.",
			Embedding:  unitVec(0),
			UpdatedAt:  time.Now().UTC(),
		},
		{
			ProjectKey: "ALPHA",
			IssueKey:   "ALPHA-2",
			Summary:    "Second issue",
			Status:     "In Progress",
			Assignee:   "bob",
			Content:    "Content of the second issue.",
			Embedding:  unitVec(1),
			UpdatedAt:  time.Now().UTC(),
		},
		{
			ProjectKey: "ALPHA",
			IssueKey:   "ALPHA-3",
			Summary:    "Third issue",
			Status:     "Done",
			Assignee:   "carol",
			Content:    "Content of the third issue.",
			Embedding:  unitVec(2),
			UpdatedAt:  time.Now().UTC(),
		},
	}

	err := st.Upsert(ctx, docs)
	require.NoError(t, err)

	// Query embedding is identical to ALPHA-1 embedding → should be top hit.
	queryEmb := unitVec(0)

	hits, err := st.Query(ctx, queryEmb, Filter{ProjectKey: "ALPHA"}, 3)
	require.NoError(t, err)
	require.Len(t, hits, 3, "expected 3 hits for topK=3")
	require.Equal(t, "ALPHA-1", hits[0].IssueKey, "ALPHA-1 must be the top hit (closest to query embedding)")
	require.NotEmpty(t, hits[0].Excerpt, "excerpt must not be empty")
}

func TestPgvectorStore_Query_EmptyWhenNoMatch(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()

	hits, err := st.Query(ctx, unitVec(0), Filter{ProjectKey: "NOSUCHPROJECT"}, 5)
	require.NoError(t, err)
	require.Empty(t, hits)
}

func TestPgvectorStore_Query_ZeroTopK(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()

	hits, err := st.Query(ctx, unitVec(0), Filter{ProjectKey: "ALPHA"}, 0)
	require.NoError(t, err)
	require.Empty(t, hits)
}
