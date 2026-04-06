//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPgvectorStore_Stats(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()

	// Empty project → 0.
	count, err := st.Stats(ctx, "STATS")
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Upsert 2 docs → Stats == 2.
	docs := []Document{
		{
			ProjectKey: "STATS",
			IssueKey:   "STATS-1",
			Summary:    "First issue",
			Status:     "Open",
			Assignee:   "alice",
			Content:    "Content one",
			Embedding:  makeEmbedding(),
			UpdatedAt:  time.Now().UTC(),
		},
		{
			ProjectKey: "STATS",
			IssueKey:   "STATS-2",
			Summary:    "Second issue",
			Status:     "In Progress",
			Assignee:   "bob",
			Content:    "Content two",
			Embedding:  makeEmbedding(),
			UpdatedAt:  time.Now().UTC(),
		},
	}

	err = st.Upsert(ctx, docs)
	require.NoError(t, err)

	count, err = st.Stats(ctx, "STATS")
	require.NoError(t, err)
	require.Equal(t, 2, count)
}
