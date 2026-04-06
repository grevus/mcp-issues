package index

import (
	"context"
	"fmt"

	"github.com/grevus/mcp-jira/internal/jira"
	"github.com/grevus/mcp-jira/internal/rag/store"
)

// IssueReader streams IssueDoc values for a given Jira project.
type IssueReader interface {
	IterateIssueDocs(ctx context.Context, projectKey string) (<-chan jira.IssueDoc, <-chan error)
}

// Embedder converts a batch of texts into dense vector representations.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Store persists indexed documents.
type Store interface {
	Upsert(ctx context.Context, docs []store.Document) error
}

// Indexer orchestrates the full reindex pipeline for a Jira project:
// read → render → embed → upsert.
type Indexer struct {
	Reader   IssueReader
	Embedder Embedder
	Store    Store
}

// New returns a new Indexer wired to the given dependencies.
func New(r IssueReader, e Embedder, s Store) *Indexer {
	return &Indexer{Reader: r, Embedder: e, Store: s}
}

// Reindex fetches all issue docs for projectKey, embeds them, and upserts into
// the store. It returns the number of documents upserted.
// If the project has no issues, it returns (0, nil) without calling Embed or Upsert.
func (idx *Indexer) Reindex(ctx context.Context, projectKey string) (int, error) {
	docsCh, errCh := idx.Reader.IterateIssueDocs(ctx, projectKey)

	var issueDocs []jira.IssueDoc
	for {
		select {
		case doc, ok := <-docsCh:
			if !ok {
				docsCh = nil
			} else {
				issueDocs = append(issueDocs, doc)
			}
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
			} else if err != nil {
				return 0, fmt.Errorf("index: reading issue docs: %w", err)
			}
		}
		if docsCh == nil && errCh == nil {
			break
		}
	}

	if len(issueDocs) == 0 {
		return 0, nil
	}

	// Render all docs into flat text and build store.Document skeletons.
	texts := make([]string, len(issueDocs))
	documents := make([]store.Document, len(issueDocs))
	for i, d := range issueDocs {
		text := RenderDoc(d)
		texts[i] = text
		documents[i] = store.Document{
			ProjectKey: d.ProjectKey,
			IssueKey:   d.Key,
			Summary:    d.Summary,
			Status:     d.Status,
			Assignee:   d.Assignee,
			Content:    text,
			UpdatedAt:  d.UpdatedAt,
		}
	}

	embeddings, err := idx.Embedder.Embed(ctx, texts)
	if err != nil {
		return 0, fmt.Errorf("index: embedding docs: %w", err)
	}
	if len(embeddings) != len(documents) {
		return 0, fmt.Errorf("index: embedder returned %d vectors for %d documents", len(embeddings), len(documents))
	}

	for i := range documents {
		documents[i].Embedding = embeddings[i]
	}

	if err := idx.Store.Upsert(ctx, documents); err != nil {
		return 0, fmt.Errorf("index: upserting docs: %w", err)
	}

	return len(documents), nil
}
