//go:build integration

package pgvector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

func TestPgvectorStore_Stats(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()
	tenantID := ""

	// Empty project → 0.
	count, err := st.Stats(ctx, tenantID, "STATS")
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Upsert 2 docs → Stats == 2.
	docs := []knowledge.Document{
		{
			TenantID:   tenantID,
			Source:     "jira",
			ProjectKey: "STATS",
			DocKey:     "STATS-1",
			Title:      "First issue",
			Status:     "Open",
			Author:     "alice",
			Content:    "Content one",
			Embedding:  makeEmbedding(),
			UpdatedAt:  time.Now().UTC(),
		},
		{
			TenantID:   tenantID,
			Source:     "jira",
			ProjectKey: "STATS",
			DocKey:     "STATS-2",
			Title:      "Second issue",
			Status:     "In Progress",
			Author:     "bob",
			Content:    "Content two",
			Embedding:  makeEmbedding(),
			UpdatedAt:  time.Now().UTC(),
		},
	}

	err = st.Upsert(ctx, docs)
	require.NoError(t, err)

	count, err = st.Stats(ctx, tenantID, "STATS")
	require.NoError(t, err)
	require.Equal(t, 2, count)
}
