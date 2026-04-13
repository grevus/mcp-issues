//go:build integration

package pgvector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

func makeEmbedding() []float32 {
	emb := make([]float32, 1024)
	emb[0] = 0.1
	emb[1] = 0.9
	return emb
}

func TestPgvectorStore_Upsert_Insert(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()

	doc := knowledge.Document{
		TenantID:   "",
		Source:     "jira",
		ProjectKey: "TEST",
		DocKey:     "TEST-1",
		Title:      "Initial summary",
		Status:     "Open",
		Author:     "alice",
		Content:    "Some content",
		Embedding:  makeEmbedding(),
		UpdatedAt:  time.Now().UTC(),
	}

	err := st.Upsert(ctx, []knowledge.Document{doc})
	require.NoError(t, err)

	var count int
	err = st.pool.QueryRow(ctx, "SELECT count(*) FROM issues_index WHERE doc_key=$1", "TEST-1").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestPgvectorStore_Upsert_Update(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()

	original := knowledge.Document{
		TenantID:   "",
		Source:     "jira",
		ProjectKey: "TEST",
		DocKey:     "TEST-2",
		Title:      "Original summary",
		Status:     "Open",
		Author:     "bob",
		Content:    "Original content",
		Embedding:  makeEmbedding(),
		UpdatedAt:  time.Now().UTC(),
	}

	err := st.Upsert(ctx, []knowledge.Document{original})
	require.NoError(t, err)

	updated := knowledge.Document{
		TenantID:   "",
		Source:     "jira",
		ProjectKey: "TEST",
		DocKey:     "TEST-2",
		Title:      "Updated summary",
		Status:     "Done",
		Author:     "bob",
		Content:    "Updated content",
		Embedding:  makeEmbedding(),
		UpdatedAt:  time.Now().UTC(),
	}

	err = st.Upsert(ctx, []knowledge.Document{updated})
	require.NoError(t, err)

	var count int
	err = st.pool.QueryRow(ctx, "SELECT count(*) FROM issues_index WHERE doc_key=$1", "TEST-2").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "must not create a duplicate row")

	var title string
	err = st.pool.QueryRow(ctx, "SELECT title FROM issues_index WHERE doc_key=$1", "TEST-2").Scan(&title)
	require.NoError(t, err)
	require.Equal(t, "Updated summary", title)
}
