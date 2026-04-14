package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

func newStore(t *testing.T) *SqliteStore {
	t.Helper()
	dir := t.TempDir()
	st, err := New(context.Background(), filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// onehot returns a unit vector of given dim with value 1 at position i.
func onehot(dim, i int) []float32 {
	v := make([]float32, dim)
	v[i%dim] = 1
	return v
}

func TestUpsertStatsSearch(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	const dim = 1024
	now := time.Now().UTC()
	docs := []knowledge.Document{
		{
			TenantID:   "t1",
			Source:     "jira",
			ProjectKey: "ABC",
			DocKey:     "ABC-1",
			Title:      "login broken",
			Status:     "Open",
			Content:    "users cannot log in with SSO",
			Embedding:  onehot(dim, 0),
			UpdatedAt:  now,
		},
		{
			TenantID:   "t1",
			Source:     "jira",
			ProjectKey: "ABC",
			DocKey:     "ABC-2",
			Title:      "payments failing",
			Status:     "In Progress",
			Content:    "stripe webhook returns 500",
			Embedding:  onehot(dim, 1),
			UpdatedAt:  now,
		},
	}
	require.NoError(t, st.Upsert(ctx, docs))

	// Stats
	n, err := st.Stats(ctx, "t1", "ABC")
	require.NoError(t, err)
	require.Equal(t, 2, n)

	// Search: query close to doc 0 should return ABC-1 first.
	hits, err := st.Search(ctx, onehot(dim, 0), knowledge.Filter{TenantID: "t1", ProjectKey: "ABC"}, 2)
	require.NoError(t, err)
	require.Len(t, hits, 2)
	require.Equal(t, "ABC-1", hits[0].DocKey)
	require.Equal(t, "login broken", hits[0].Title)
	require.InDelta(t, 1.0, hits[0].Score, 0.001)
	require.Equal(t, "ABC-2", hits[1].DocKey)
}

func TestUpsertUpdatesExisting(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	const dim = 1024

	doc := knowledge.Document{
		TenantID: "t1", Source: "jira", ProjectKey: "ABC", DocKey: "ABC-1",
		Title: "old title", Content: "old content",
		Embedding: onehot(dim, 0), UpdatedAt: time.Now(),
	}
	require.NoError(t, st.Upsert(ctx, []knowledge.Document{doc}))

	// Update same doc_key — should replace.
	doc.Title = "new title"
	doc.Content = "new content"
	doc.Embedding = onehot(dim, 5)
	require.NoError(t, st.Upsert(ctx, []knowledge.Document{doc}))

	n, err := st.Stats(ctx, "t1", "ABC")
	require.NoError(t, err)
	require.Equal(t, 1, n)

	hits, err := st.Search(ctx, onehot(dim, 5), knowledge.Filter{TenantID: "t1", ProjectKey: "ABC"}, 1)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, "new title", hits[0].Title)
}

func TestReplaceProject(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	const dim = 1024
	now := time.Now()

	initial := []knowledge.Document{
		{TenantID: "t1", Source: "jira", ProjectKey: "ABC", DocKey: "ABC-1", Title: "a", Embedding: onehot(dim, 0), UpdatedAt: now},
		{TenantID: "t1", Source: "jira", ProjectKey: "ABC", DocKey: "ABC-2", Title: "b", Embedding: onehot(dim, 1), UpdatedAt: now},
	}
	require.NoError(t, st.Upsert(ctx, initial))

	// Replace with a smaller set — old ones must be gone.
	replacement := []knowledge.Document{
		{TenantID: "t1", Source: "jira", ProjectKey: "ABC", DocKey: "ABC-9", Title: "z", Embedding: onehot(dim, 9), UpdatedAt: now},
	}
	require.NoError(t, st.ReplaceProject(ctx, "t1", "ABC", replacement))

	n, err := st.Stats(ctx, "t1", "ABC")
	require.NoError(t, err)
	require.Equal(t, 1, n)

	hits, err := st.Search(ctx, onehot(dim, 9), knowledge.Filter{TenantID: "t1", ProjectKey: "ABC"}, 5)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, "ABC-9", hits[0].DocKey)
}

func TestFilterByTenantAndProject(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	const dim = 1024
	now := time.Now()

	docs := []knowledge.Document{
		{TenantID: "t1", Source: "jira", ProjectKey: "ABC", DocKey: "ABC-1", Title: "t1/abc", Embedding: onehot(dim, 0), UpdatedAt: now},
		{TenantID: "t1", Source: "jira", ProjectKey: "XYZ", DocKey: "XYZ-1", Title: "t1/xyz", Embedding: onehot(dim, 0), UpdatedAt: now},
		{TenantID: "t2", Source: "jira", ProjectKey: "ABC", DocKey: "ABC-1", Title: "t2/abc", Embedding: onehot(dim, 0), UpdatedAt: now},
	}
	require.NoError(t, st.Upsert(ctx, docs))

	hits, err := st.Search(ctx, onehot(dim, 0), knowledge.Filter{TenantID: "t1", ProjectKey: "ABC"}, 10)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, "t1/abc", hits[0].Title)
}

func TestSearchEmpty(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	hits, err := st.Search(ctx, []float32{}, knowledge.Filter{TenantID: "t1", ProjectKey: "ABC"}, 10)
	require.NoError(t, err)
	require.Empty(t, hits)

	hits, err = st.Search(ctx, onehot(1024, 0), knowledge.Filter{TenantID: "t1", ProjectKey: "ABC"}, 0)
	require.NoError(t, err)
	require.Empty(t, hits)
}
