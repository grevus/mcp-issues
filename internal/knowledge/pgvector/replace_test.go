//go:build integration

package pgvector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

func TestPgvectorStore_ReplaceProject(t *testing.T) {
	st, cleanup := setupPgvector(t)
	defer cleanup()

	ctx := context.Background()
	tenantID := ""
	projectKey := "REPL"

	makeDoc := func(key, title string) knowledge.Document {
		return knowledge.Document{
			TenantID:   tenantID,
			Source:     "jira",
			ProjectKey: projectKey,
			DocKey:     key,
			Title:      title,
			Status:     "Open",
			Author:     "tester",
			Content:    "content for " + key,
			Embedding:  makeEmbedding(),
			UpdatedAt:  time.Now().UTC(),
		}
	}

	// Step 1: insert 2 documents via Upsert.
	err := st.Upsert(ctx, []knowledge.Document{
		makeDoc("REPL-1", "First"),
		makeDoc("REPL-2", "Second"),
	})
	require.NoError(t, err)

	count, err := st.Stats(ctx, tenantID, projectKey)
	require.NoError(t, err)
	require.Equal(t, 2, count, "после Upsert должно быть 2 документа")

	// Step 2: ReplaceProject with only 1 document — should atomically delete
	// the old 2 and insert the new 1.
	err = st.ReplaceProject(ctx, tenantID, projectKey, []knowledge.Document{
		makeDoc("REPL-3", "Replacement"),
	})
	require.NoError(t, err)

	count, err = st.Stats(ctx, tenantID, projectKey)
	require.NoError(t, err)
	require.Equal(t, 1, count, "после ReplaceProject должен остаться ровно 1 документ")

	// Verify it is the new document, not one of the old ones.
	var docKey string
	err = st.pool.QueryRow(ctx,
		"SELECT doc_key FROM issues_index WHERE tenant_id=$1 AND project_key=$2", tenantID, projectKey,
	).Scan(&docKey)
	require.NoError(t, err)
	require.Equal(t, "REPL-3", docKey, "должен остаться REPL-3, а не старые REPL-1/REPL-2")
}
