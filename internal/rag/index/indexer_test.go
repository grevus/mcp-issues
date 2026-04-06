package index

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grevus/mcp-jira/internal/jira"
	"github.com/grevus/mcp-jira/internal/rag/store"
)

// fakeReader emits a fixed list of IssueDoc values and then closes both channels.
type fakeReader struct {
	docs []jira.IssueDoc
}

func (r *fakeReader) IterateIssueDocs(_ context.Context, _ string) (<-chan jira.IssueDoc, <-chan error) {
	docsCh := make(chan jira.IssueDoc, len(r.docs))
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
type fakeEmbedder struct {
	dim int
}

func (e *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
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

// fakeStore records the documents passed to Upsert.
type fakeStore struct {
	upserted []store.Document
}

func (s *fakeStore) Upsert(_ context.Context, docs []store.Document) error {
	s.upserted = append(s.upserted, docs...)
	return nil
}

func TestReindex_HappyPath(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	issueDocs := []jira.IssueDoc{
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
	st := &fakeStore{}

	idx := New(reader, embedder, st)
	count, err := idx.Reindex(context.Background(), "ABC")

	require.NoError(t, err)
	require.Equal(t, 3, count, "должно быть проиндексировано 3 документа")
	require.Len(t, st.upserted, 3, "Upsert должен получить 3 документа")

	for i, doc := range st.upserted {
		require.Equal(t, "ABC", doc.ProjectKey, "doc[%d]: project_key", i)
		require.Equal(t, issueDocs[i].Key, doc.IssueKey, "doc[%d]: issue_key", i)
		require.NotEmpty(t, doc.Content, "doc[%d]: content не должен быть пустым", i)
		require.NotEmpty(t, doc.Embedding, "doc[%d]: embedding не должен быть пустым", i)

		// Проверяем, что embedding соответствует индексу (fakeEmbedder: все элементы = float32(i+1))
		expectedVal := float32(i + 1)
		for j, v := range doc.Embedding {
			require.Equal(t, expectedVal, v, "doc[%d] embedding[%d]", i, j)
		}
	}
}

func TestReindex_EmptyProject(t *testing.T) {
	reader := &fakeReader{docs: nil}
	embedder := &fakeEmbedder{dim: 4}
	st := &fakeStore{}

	idx := New(reader, embedder, st)
	count, err := idx.Reindex(context.Background(), "EMPTY")

	require.NoError(t, err)
	require.Equal(t, 0, count)
	require.Empty(t, st.upserted, "Upsert не должен вызываться для пустого проекта")
}
