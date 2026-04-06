package retriever_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grevus/mcp-jira/internal/rag/retriever"
	"github.com/grevus/mcp-jira/internal/rag/store"
)

// fakeEmbedder records the texts it received and returns a fixed embedding.
type fakeEmbedder struct {
	texts []string
	vecs  [][]float32
	err   error
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	f.texts = texts
	return f.vecs, f.err
}

// fakeStore records the arguments it received and returns fixed hits.
type fakeStore struct {
	gotEmbedding []float32
	gotFilter    store.Filter
	gotTopK      int
	hits         []store.Hit
	err          error
}

func (f *fakeStore) Query(_ context.Context, queryEmbedding []float32, filter store.Filter, topK int) ([]store.Hit, error) {
	f.gotEmbedding = queryEmbedding
	f.gotFilter = filter
	f.gotTopK = topK
	return f.hits, f.err
}

func TestSearch_HappyPath(t *testing.T) {
	wantEmbedding := []float32{0.1, 0.2, 0.3}
	wantHits := []store.Hit{
		{IssueKey: "ABC-1", Summary: "Do the thing", Status: "In Progress", Score: 0.9, Excerpt: "some text"},
		{IssueKey: "ABC-2", Summary: "Another thing", Status: "Done", Score: 0.7, Excerpt: "other text"},
	}

	emb := &fakeEmbedder{vecs: [][]float32{wantEmbedding}}
	st := &fakeStore{hits: wantHits}

	r := retriever.New(emb, st)
	hits, err := r.Search(context.Background(), "ABC", "do the thing", 5)

	require.NoError(t, err)
	require.Equal(t, wantHits, hits)

	// Embedder received the right query text.
	require.Equal(t, []string{"do the thing"}, emb.texts)

	// Store received the embedding from Embedder.
	require.Equal(t, wantEmbedding, st.gotEmbedding)

	// Store received the correct filter.
	require.Equal(t, store.Filter{ProjectKey: "ABC"}, st.gotFilter)

	// Store received the correct topK.
	require.Equal(t, 5, st.gotTopK)
}

func TestSearch_EmbedError(t *testing.T) {
	wantErr := errors.New("embed failed")
	emb := &fakeEmbedder{err: wantErr}
	st := &fakeStore{}

	r := retriever.New(emb, st)
	hits, err := r.Search(context.Background(), "ABC", "query", 3)

	require.Nil(t, hits)
	require.ErrorIs(t, err, wantErr)
	require.Contains(t, err.Error(), "retriever: Search:")
}

func TestSearch_EmptyEmbeddings(t *testing.T) {
	emb := &fakeEmbedder{vecs: [][]float32{}} // empty slice, no error
	st := &fakeStore{}

	r := retriever.New(emb, st)
	hits, err := r.Search(context.Background(), "ABC", "query", 3)

	require.Nil(t, hits)
	require.EqualError(t, err, "retriever: Search: empty embeddings")
}

func TestSearch_StoreError(t *testing.T) {
	wantErr := errors.New("db down")
	emb := &fakeEmbedder{vecs: [][]float32{{0.5}}}
	st := &fakeStore{err: wantErr}

	r := retriever.New(emb, st)
	hits, err := r.Search(context.Background(), "ABC", "query", 10)

	require.Nil(t, hits)
	require.ErrorIs(t, err, wantErr)
	require.Contains(t, err.Error(), "retriever: Search:")
}
