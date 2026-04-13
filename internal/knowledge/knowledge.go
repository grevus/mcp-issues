package knowledge

import (
	"context"
	"time"
)

// Document — единица знания для индексации. Tracker-agnostic.
type Document struct {
	TenantID   string
	Source     string // "jira", "youtrack", "confluence", ...
	ProjectKey string
	DocKey     string // PROJ-123, page-id, etc.
	Title      string
	Status     string
	Author     string
	Content    string
	Embedding  []float32
	UpdatedAt  time.Time
}

// Hit — результат поиска.
type Hit struct {
	DocKey  string  `json:"doc_key"`
	Title   string  `json:"title"`
	Status  string  `json:"status"`
	Score   float32 `json:"score"`
	Excerpt string  `json:"excerpt"`
}

// Filter ограничивает поиск.
type Filter struct {
	TenantID   string
	ProjectKey string
	Source     string // опционально: "jira", "confluence", "" = все
}

// Writer — write path: индексация документов.
type Writer interface {
	Upsert(ctx context.Context, docs []Document) error
	ReplaceProject(ctx context.Context, tenantID, projectKey string, docs []Document) error
}

// Reader — read path: поиск по базе знаний.
type Reader interface {
	Search(ctx context.Context, queryEmbedding []float32, f Filter, topK int) ([]Hit, error)
}

// Store — полный интерфейс хранилища знаний.
type Store interface {
	Writer
	Reader
	Stats(ctx context.Context, tenantID, projectKey string) (int, error)
	Close() error
}
