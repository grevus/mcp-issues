package store

import (
	"context"
	"time"
)

// Document represents an indexed Jira issue ready for vector search.
type Document struct {
	ProjectKey string
	IssueKey   string
	Summary    string
	Status     string
	Assignee   string
	Content    string    // flat text used for indexing (rendered by Indexer)
	Embedding  []float32
	UpdatedAt  time.Time
}

// Filter restricts a Query to a specific Jira project.
type Filter struct {
	ProjectKey string
}

// Hit is a single result returned by Store.Query.
type Hit struct {
	IssueKey string
	Summary  string
	Status   string
	Score    float32 // cosine similarity, 0..1
	Excerpt  string  // first ~300 characters of Content
}

// Store is the interface for persisting and querying indexed issue embeddings.
type Store interface {
	// Upsert inserts or updates the given documents in the store.
	Upsert(ctx context.Context, docs []Document) error

	// Query returns the topK nearest documents to queryEmbedding filtered by f.
	Query(ctx context.Context, queryEmbedding []float32, f Filter, topK int) ([]Hit, error)

	// Stats returns the number of indexed documents for the given project key.
	Stats(ctx context.Context, projectKey string) (int, error)

	// Close releases resources held by the store.
	Close() error
}

// TxStore extends Store with a transactional bulk-replace operation.
type TxStore interface {
	Store

	// ReplaceProject atomically removes all documents for projectKey and
	// inserts docs in a single transaction. If docs is empty, only the
	// DELETE is executed (effectively wiping the project index).
	ReplaceProject(ctx context.Context, projectKey string, docs []Document) error
}
