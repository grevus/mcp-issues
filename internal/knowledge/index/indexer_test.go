package index

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/tracker"
)

// fakeReader emits a fixed list of IssueDoc values and then closes both channels.
type fakeReader struct {
	docs []tracker.IssueDoc
}

func (r *fakeReader) IterateIssueDocs(_ context.Context, _ string) (<-chan tracker.IssueDoc, <-chan error) {
	docsCh := make(chan tracker.IssueDoc, len(r.docs))
	errCh := make(chan error, 1)
	for _, d := range r.docs {
		docsCh <- d
	}
	close(docsCh)
	close(errCh)
	return docsCh, errCh
}

// fakeEmbedder returns a synthetic embedding for each input text.
// The embedding for index i is a single float32 slice where all elements equal float32(i+1).
// It records the number of Embed calls made.
type fakeEmbedder struct {
	dim   int
	calls int // number of times Embed was called
}

func (e *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	e.calls++
	out := make([][]float32, len(texts))
	for i := range texts {
		vec := make([]float32, e.dim)
		for j := range vec {
			vec[j] = float32(i + 1)
		}
		out[i] = vec
	}
	return out, nil
}

// fakeTxStore records the tenantID, projectKey and documents passed to ReplaceProject.
type fakeTxStore struct {
	replaceCalls int
	lastTenantID string
	lastProject  string
	replaced     []knowledge.Document
}

func (s *fakeTxStore) ReplaceProject(_ context.Context, tenantID, projectKey string, docs []knowledge.Document) error {
	s.replaceCalls++
	s.lastTenantID = tenantID
	s.lastProject = projectKey
	s.replaced = append([]knowledge.Document(nil), docs...) // snapshot
	return nil
}

func makeIssueDocs(projectKey string, n int) []tracker.IssueDoc {
	now := time.Now().UTC().Truncate(time.Second)
	docs := make([]tracker.IssueDoc, n)
	for i := range docs {
		docs[i] = tracker.IssueDoc{
			ProjectKey:  projectKey,
			Key:         projectKey + "-" + itoa(i+1),
			Summary:     "Issue " + itoa(i+1),
			Status:      "Open",
			Description: "Description " + itoa(i+1),
			UpdatedAt:   now,
		}
	}
	return docs
}

// itoa is a minimal int-to-string helper to avoid importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

func TestReindex_HappyPath(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	issueDocs := []tracker.IssueDoc{
		{
			ProjectKey:  "ABC",
			Key:         "ABC-1",
			Summary:     "First issue",
			Status:      "To Do",
			Assignee:    "alice",
			Description: "Description one.",
			UpdatedAt:   now,
		},
		{
			ProjectKey:  "ABC",
			Key:         "ABC-2",
			Summary:     "Second issue",
			Status:      "In Progress",
			Assignee:    "bob",
			Description: "Description two.",
			UpdatedAt:   now,
		},
		{
			ProjectKey:  "ABC",
			Key:         "ABC-3",
			Summary:     "Third issue",
			Status:      "Done",
			Description: "Description three.",
			UpdatedAt:   now,
		},
	}

	reader := &fakeReader{docs: issueDocs}
	embedder := &fakeEmbedder{dim: 4}
	st := &fakeTxStore{}

	idx := New(reader, embedder, st)
	count, err := idx.Reindex(context.Background(), "tenant1", "jira", "ABC")

	require.NoError(t, err)
	require.Equal(t, 3, count, "должно быть проиндексировано 3 документа")

	require.Equal(t, 1, st.replaceCalls, "ReplaceProject должен вызываться ровно один раз")
	require.Equal(t, "tenant1", st.lastTenantID)
	require.Equal(t, "ABC", st.lastProject)
	require.Len(t, st.replaced, 3, "ReplaceProject должен получить 3 документа")

	for i, doc := range st.replaced {
		require.Equal(t, "ABC", doc.ProjectKey, "doc[%d]: project_key", i)
		require.Equal(t, issueDocs[i].Key, doc.DocKey, "doc[%d]: doc_key", i)
		require.Equal(t, issueDocs[i].Summary, doc.Title, "doc[%d]: title", i)
		require.Equal(t, "tenant1", doc.TenantID, "doc[%d]: tenant_id", i)
		require.Equal(t, "jira", doc.Source, "doc[%d]: source", i)
		require.NotEmpty(t, doc.Content, "doc[%d]: content не должен быть пустым", i)
		require.NotEmpty(t, doc.Embedding, "doc[%d]: embedding не должен быть пустым", i)

		// Проверяем, что embedding соответствует индексу внутри батча.
		// При 3 документах (< embedBatchSize) — один батч, индексы 0..2.
		expectedVal := float32(i + 1)
		for j, v := range doc.Embedding {
			require.Equal(t, expectedVal, v, "doc[%d] embedding[%d]", i, j)
		}
	}
}

func TestReindex_EmptyProject(t *testing.T) {
	reader := &fakeReader{docs: nil}
	embedder := &fakeEmbedder{dim: 4}
	st := &fakeTxStore{}

	idx := New(reader, embedder, st)
	count, err := idx.Reindex(context.Background(), "tenant1", "jira", "EMPTY")

	require.NoError(t, err)
	require.Equal(t, 0, count)
	require.Equal(t, 0, st.replaceCalls, "ReplaceProject не должен вызываться для пустого проекта")
	require.Empty(t, st.replaced)
}

// TestReindex_ReplaceProject_CalledOnce verifies that ReplaceProject is called
// exactly once regardless of the number of documents.
func TestReindex_ReplaceProject_CalledOnce(t *testing.T) {
	docs := makeIssueDocs("PROJ", 5)
	reader := &fakeReader{docs: docs}
	embedder := &fakeEmbedder{dim: 4}
	st := &fakeTxStore{}

	idx := New(reader, embedder, st)
	count, err := idx.Reindex(context.Background(), "tenant1", "jira", "PROJ")

	require.NoError(t, err)
	require.Equal(t, 5, count)
	require.Equal(t, 1, st.replaceCalls, "ReplaceProject должен вызываться ровно один раз")
	require.Equal(t, "PROJ", st.lastProject)
	require.Len(t, st.replaced, 5)
}

// TestReindex_EmbedBatching verifies that 250 documents produce exactly 3 Embed
// calls (batches of 100 + 100 + 50).
func TestReindex_EmbedBatching(t *testing.T) {
	const total = 250
	docs := makeIssueDocs("BAT", total)
	reader := &fakeReader{docs: docs}
	embedder := &fakeEmbedder{dim: 4}
	st := &fakeTxStore{}

	idx := New(reader, embedder, st)
	count, err := idx.Reindex(context.Background(), "tenant1", "jira", "BAT")

	require.NoError(t, err)
	require.Equal(t, total, count)
	require.Equal(t, 3, embedder.calls, "250 docs / batch 100 → 3 вызова Embed (100+100+50)")
	require.Equal(t, 1, st.replaceCalls, "ReplaceProject вызывается один раз")
	require.Len(t, st.replaced, total)
}
