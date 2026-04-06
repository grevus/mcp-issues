package retriever

import (
	"context"
	"fmt"

	"github.com/grevus/mcp-jira/internal/rag/store"
)

// Embedder converts texts into dense vector representations.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Store is the subset of store.Store required by Retriever.
type Store interface {
	Query(ctx context.Context, queryEmbedding []float32, f store.Filter, topK int) ([]store.Hit, error)
}

// Retriever performs semantic search over indexed Jira issues.
type Retriever struct {
	Embedder Embedder
	Store    Store
}

// New constructs a Retriever backed by e and s.
func New(e Embedder, s Store) *Retriever {
	return &Retriever{Embedder: e, Store: s}
}

// Search embeds query and returns the topK most similar issues in projectKey.
func (r *Retriever) Search(ctx context.Context, projectKey, query string, topK int) ([]store.Hit, error) {
	vecs, err := r.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("retriever: Search: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("retriever: Search: empty embeddings")
	}

	hits, err := r.Store.Query(ctx, vecs[0], store.Filter{ProjectKey: projectKey}, topK)
	if err != nil {
		return nil, fmt.Errorf("retriever: Search: %w", err)
	}
	return hits, nil
}
