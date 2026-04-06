//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

	doc := Document{
		ProjectKey: "TEST",
		IssueKey:   "TEST-1",
		Summary:    "Initial summary",
		Status:     "Open",
		Assignee:   "alice",
		Content:    "Some content",
		Embedding:  makeEmbedding(),
		UpdatedAt:  time.Now().UTC(),
	}

	err := st.Upsert(ctx, []Document{doc})
	require.NoError(t, err)

	var count int
	err = st.pool.QueryRow(ctx, "SELECT count(*) FROM issues_index WHERE issue_key=$1", "TEST-1").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestPgvectorStore_Upsert_Update(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()

	original := Document{
		ProjectKey: "TEST",
		IssueKey:   "TEST-2",
		Summary:    "Original summary",
		Status:     "Open",
		Assignee:   "bob",
		Content:    "Original content",
		Embedding:  makeEmbedding(),
		UpdatedAt:  time.Now().UTC(),
	}

	err := st.Upsert(ctx, []Document{original})
	require.NoError(t, err)

	updated := Document{
		ProjectKey: "TEST",
		IssueKey:   "TEST-2",
		Summary:    "Updated summary",
		Status:     "Done",
		Assignee:   "bob",
		Content:    "Updated content",
		Embedding:  makeEmbedding(),
		UpdatedAt:  time.Now().UTC(),
	}

	err = st.Upsert(ctx, []Document{updated})
	require.NoError(t, err)

	var count int
	err = st.pool.QueryRow(ctx, "SELECT count(*) FROM issues_index WHERE issue_key=$1", "TEST-2").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "must not create a duplicate row")

	var summary string
	err = st.pool.QueryRow(ctx, "SELECT summary FROM issues_index WHERE issue_key=$1", "TEST-2").Scan(&summary)
	require.NoError(t, err)
	require.Equal(t, "Updated summary", summary)
}
