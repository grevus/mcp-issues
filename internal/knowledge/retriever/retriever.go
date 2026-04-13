package retriever

import (
	"context"
	"fmt"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

// Embedder converts texts into dense vector representations.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Store is the subset of knowledge.Store required by Retriever.
type Store interface {
	Search(ctx context.Context, queryEmbedding []float32, f knowledge.Filter, topK int) ([]knowledge.Hit, error)
}

// Retriever performs semantic search over indexed documents.
type Retriever struct {
	Embedder Embedder
	Store    Store
	TenantID string
}

// New constructs a Retriever backed by e and s for the given tenantID.
func New(e Embedder, s Store, tenantID string) *Retriever {
	return &Retriever{Embedder: e, Store: s, TenantID: tenantID}
}

// Search embeds query and returns the topK most similar documents in projectKey.
func (r *Retriever) Search(ctx context.Context, projectKey, query string, topK int) ([]knowledge.Hit, error) {
	vecs, err := r.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("retriever: Search: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("retriever: Search: empty embeddings")
	}

	hits, err := r.Store.Search(ctx, vecs[0], knowledge.Filter{
		TenantID:   r.TenantID,
		ProjectKey: projectKey,
	}, topK)
	if err != nil {
		return nil, fmt.Errorf("retriever: Search: %w", err)
	}
	return hits, nil
}
