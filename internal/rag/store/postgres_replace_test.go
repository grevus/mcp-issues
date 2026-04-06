//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPgvectorStore_ReplaceProject(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()
	projectKey := "REPL"

	makeDoc := func(key, summary string) Document {
		return Document{
			ProjectKey: projectKey,
			IssueKey:   key,
			Summary:    summary,
			Status:     "Open",
			Assignee:   "tester",
			Content:    "content for " + key,
			Embedding:  makeEmbedding(),
			UpdatedAt:  time.Now().UTC(),
		}
	}

	// Step 1: insert 2 documents via Upsert.
	err := st.Upsert(ctx, []Document{
		makeDoc("REPL-1", "First"),
		makeDoc("REPL-2", "Second"),
	})
	require.NoError(t, err)

	count, err := st.Stats(ctx, projectKey)
	require.NoError(t, err)
	require.Equal(t, 2, count, "после Upsert должно быть 2 документа")

	// Step 2: ReplaceProject with only 1 document — should atomically delete
	// the old 2 and insert the new 1.
	err = st.ReplaceProject(ctx, projectKey, []Document{
		makeDoc("REPL-3", "Replacement"),
	})
	require.NoError(t, err)

	count, err = st.Stats(ctx, projectKey)
	require.NoError(t, err)
	require.Equal(t, 1, count, "после ReplaceProject должен остаться ровно 1 документ")

	// Verify it is the new document, not one of the old ones.
	var issueKey string
	err = st.pool.QueryRow(ctx,
		"SELECT issue_key FROM issues_index WHERE project_key=$1", projectKey,
	).Scan(&issueKey)
	require.NoError(t, err)
	require.Equal(t, "REPL-3", issueKey, "должен остаться REPL-3, а не старые REPL-1/REPL-2")
}
