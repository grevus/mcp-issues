package index

import (
	"context"
	"fmt"

	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/tracker"
)

const embedBatchSize = 100

// IssueReader streams IssueDoc values for a given project.
type IssueReader interface {
	IterateIssueDocs(ctx context.Context, projectKey string) (<-chan tracker.IssueDoc, <-chan error)
}

// Embedder converts a batch of texts into dense vector representations.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Store persists indexed documents with transactional replace support.
type Store interface {
	ReplaceProject(ctx context.Context, tenantID, projectKey string, docs []knowledge.Document) error
}

// Indexer orchestrates the full reindex pipeline for a project:
// read → render → embed (batched) → ReplaceProject.
type Indexer struct {
	Reader   IssueReader
	Embedder Embedder
	Store    Store
}

// New returns a new Indexer wired to the given dependencies.
func New(r IssueReader, e Embedder, s Store) *Indexer {
	return &Indexer{Reader: r, Embedder: e, Store: s}
}

// Reindex fetches all issue docs for projectKey, embeds them in batches of
// embedBatchSize, and atomically replaces the project's index via
// Store.ReplaceProject. It returns the number of documents indexed.
// If the project has no issues, it returns (0, nil) without calling Embed or
// ReplaceProject.
func (idx *Indexer) Reindex(ctx context.Context, tenantID, source, projectKey string) (int, error) {
	docsCh, errCh := idx.Reader.IterateIssueDocs(ctx, projectKey)

	var issueDocs []tracker.IssueDoc
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

	// Render all docs into flat text and build knowledge.Document skeletons.
	texts := make([]string, len(issueDocs))
	documents := make([]knowledge.Document, len(issueDocs))
	for i, d := range issueDocs {
		text := RenderDoc(d)
		texts[i] = text
		documents[i] = knowledge.Document{
			TenantID:   tenantID,
			Source:     source,
			ProjectKey: d.ProjectKey,
			DocKey:     d.Key,
			Title:      d.Summary,
			Status:     d.Status,
			Author:     d.Assignee,
			Content:    text,
			UpdatedAt:  d.UpdatedAt,
		}
	}

	// Embed in batches of embedBatchSize and stitch results together.
	allEmbeddings := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += embedBatchSize {
		end := start + embedBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch, err := idx.Embedder.Embed(ctx, texts[start:end])
		if err != nil {
			return 0, fmt.Errorf("index: embedding batch [%d:%d]: %w", start, end, err)
		}
		if len(batch) != end-start {
			return 0, fmt.Errorf("index: embedder returned %d vectors for batch of %d", len(batch), end-start)
		}
		allEmbeddings = append(allEmbeddings, batch...)
	}

	for i := range documents {
		documents[i].Embedding = allEmbeddings[i]
	}

	if err := idx.Store.ReplaceProject(ctx, tenantID, projectKey, documents); err != nil {
		return 0, fmt.Errorf("index: replacing project docs: %w", err)
	}

	return len(documents), nil
}
