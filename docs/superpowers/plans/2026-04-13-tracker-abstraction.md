# Tracker Abstraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ввести tracker-agnostic интерфейсы для задач и базы знаний, чтобы MCP-сервер работал с любым трекером (Jira, YouTrack, Linear) и поддерживал multi-tenant режим.

**Architecture:** Новые пакеты `internal/tracker/` (domain types + интерфейсы), `internal/knowledge/` (tracker-agnostic RAG), `internal/tenant/` (multi-tenant registry). Текущий `internal/jira/` переезжает в `internal/tracker/jira/`, `internal/rag/` — в `internal/knowledge/`. Handlers переключаются на `tracker.*` типы. Register получает `adaptTenant` для per-request резолвинга трекера.

**Tech Stack:** Go 1.26+, pgvector, goose migrations, gopkg.in/yaml.v3

---

## Файловая структура

### Новые файлы
- `internal/tracker/tracker.go` — domain types + интерфейсы (Issue, ListParams, SprintHealth, SprintReport, IssueDoc, Provider)
- `internal/tracker/jira/client_base.go` — перенос из `internal/jira/client_base.go`
- `internal/tracker/jira/client_issues.go` — перенос, возврат `tracker.Issue`
- `internal/tracker/jira/client_sprints.go` — перенос, возврат `tracker.SprintHealth`/`SprintReport`
- `internal/tracker/jira/client_docs.go` — перенос, возврат `tracker.IssueDoc`
- `internal/tracker/jira/jql.go` — перенос без изменений
- `internal/knowledge/knowledge.go` — Document, Hit, Filter, Store interfaces
- `internal/knowledge/pgvector/store.go` — перенос из `internal/rag/store/postgres.go`
- `internal/knowledge/pgvector/query.go` — перенос, tenant_id+source фильтрация
- `internal/knowledge/pgvector/upsert.go` — перенос, tenant_id+source поля
- `internal/knowledge/pgvector/replace.go` — перенос, tenant_id фильтрация
- `internal/knowledge/pgvector/stats.go` — перенос, tenant_id фильтрация
- `internal/knowledge/pgvector/migrations.go` — перенос + новая миграция 002
- `internal/knowledge/pgvector/migrations/001_init.sql` — без изменений
- `internal/knowledge/pgvector/migrations/002_multi_tenant.sql` — tenant_id, source, doc_key
- `internal/knowledge/embed/` — перенос из `internal/rag/embed/` без изменений
- `internal/knowledge/index/indexer.go` — перенос, `tracker.IssueDoc` вместо `jira.IssueDoc`
- `internal/knowledge/index/render.go` — перенос, `tracker.IssueDoc`
- `internal/knowledge/retriever/retriever.go` — перенос, `knowledge.Filter` вместо `store.Filter`
- `internal/tenant/tenant.go` — Config, Tenant, Registry
- `internal/tenant/loader.go` — LoadTenantsFromFile(path)

### Модифицируемые файлы
- `internal/handlers/issues.go` — `jira.*` → `tracker.*`
- `internal/handlers/sprints.go` — `jira.*` → `tracker.*`
- `internal/handlers/similar_issues.go` — `jira.*` → `tracker.*`
- `internal/handlers/standup_digest.go` — `jira.*` → `tracker.*`
- `internal/handlers/sprint_report.go` — `jira.*` → `tracker.*`
- `internal/handlers/incident_context.go` — `jira.*` → `tracker.*`
- `internal/handlers/ticket_triage.go` — `jira.*` → `tracker.*`
- `internal/handlers/release_risk_check.go` — `jira.*` → `tracker.*`
- `internal/handlers/knowledge.go` — `Hit` → `knowledge.Hit`
- `internal/register/register.go` — `adaptTenant`, убрать `JiraClient`
- `internal/register/adapt.go` — добавить `adaptTenant`
- `internal/config/config.go` — убрать Jira env из обязательных для http+keysfile
- `cmd/server/main.go` — Registry wiring
- `cmd/index/main.go` — `--tenant` flag

### Удаляемые файлы (после переноса)
- `internal/jira/` — весь пакет
- `internal/rag/` — весь пакет

---

### Task 1: Создать `internal/tracker/tracker.go` — domain types + интерфейсы

**Files:**
- Create: `internal/tracker/tracker.go`
- Test: компиляция

- [ ] **Step 1: Создать файл с domain types**

```go
package tracker

import (
	"context"
	"time"
)

// Issue — задача из любого трекера.
type Issue struct {
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Assignee string `json:"assignee,omitempty"`
}

// ListParams — параметры поиска задач.
type ListParams struct {
	ProjectKey  string
	Status      string
	Assignee    string
	FixVersion  string
	UpdatedFrom string // "YYYY-MM-DD" или "YYYY-MM-DD HH:MM"
	UpdatedTo   string
	Limit       int
}

// SprintHealth — метрики здоровья спринта.
type SprintHealth struct {
	BoardID    int     `json:"board_id"`
	SprintName string  `json:"sprint_name"`
	Total      int     `json:"total"`
	Done       int     `json:"done"`
	InProgress int     `json:"in_progress"`
	Blocked    int     `json:"blocked"`
	Velocity   float64 `json:"velocity"`
}

// SprintReport — расширенный health-отчёт спринта.
type SprintReport struct {
	Health        SprintHealth `json:"health"`
	BlockedIssues []Issue      `json:"blocked_issues"`
	ScopeAdded    []Issue      `json:"scope_added"`
	ScopeRemoved  []Issue      `json:"scope_removed"`
}

// IssueDoc — полный документ задачи для индексации в RAG.
type IssueDoc struct {
	ProjectKey    string
	Key           string
	Summary       string
	Status        string
	Assignee      string
	Description   string
	Comments      []string
	StatusHistory []string
	LinkedIssues  []string
	UpdatedAt     time.Time
}

// --- Узкие интерфейсы ---

// IssueLister — поиск задач по параметрам.
type IssueLister interface {
	ListIssues(ctx context.Context, p ListParams) ([]Issue, error)
}

// IssueFetcher — получить одну задачу по ключу + описание.
type IssueFetcher interface {
	GetIssue(ctx context.Context, key string) (Issue, string, error)
}

// SprintReader — здоровье активного спринта.
type SprintReader interface {
	GetSprintHealth(ctx context.Context, boardID int) (SprintHealth, error)
}

// SprintReporter — расширенный отчёт спринта.
type SprintReporter interface {
	GetSprintReport(ctx context.Context, boardID, sprintID int) (SprintReport, error)
}

// ScopeReader — scope changes спринта через changelog.
type ScopeReader interface {
	GetSprintScopeChanges(ctx context.Context, sprintID int) (added, removed []string, err error)
}

// CommentFetcher — комментарии задачи.
type CommentFetcher interface {
	GetIssueComments(ctx context.Context, issueKey string) ([]string, error)
}

// DocIterator — итерация всех документов проекта для индексации.
type DocIterator interface {
	IterateIssueDocs(ctx context.Context, projectKey string) (<-chan IssueDoc, <-chan error)
}

// Provider — суперинтерфейс для wiring. Каждый трекер реализует.
// handlers получают узкие интерфейсы (ISP), не Provider целиком.
type Provider interface {
	IssueLister
	IssueFetcher
	SprintReader
	SprintReporter
	ScopeReader
	CommentFetcher
	DocIterator
}
```

- [ ] **Step 2: Проверить компиляцию**

Run: `cd /Users/avlobtsov/go/src/github.com/grevus/mcp-jira && go build ./internal/tracker/`
Expected: SUCCESS, no errors

- [ ] **Step 3: Commit**

```bash
git add internal/tracker/tracker.go
git commit -m "feat: add internal/tracker package with domain types and interfaces"
```

---

### Task 2: Перенести `internal/jira/` → `internal/tracker/jira/`

**Files:**
- Create: `internal/tracker/jira/client_base.go`, `client_issues.go`, `client_sprints.go`, `client_docs.go`, `jql.go`
- Copy tests: все `*_test.go` файлы

Это механический перенос с заменой `jira.Issue` → `tracker.Issue` и т.д. в возвращаемых типах. Внутренние DTO (приватные типы для парсинга JSON) остаются в пакете `jira`.

- [ ] **Step 1: Скопировать все файлы**

```bash
mkdir -p internal/tracker/jira
cp internal/jira/client_base.go internal/tracker/jira/
cp internal/jira/client_issues.go internal/tracker/jira/
cp internal/jira/client_sprints.go internal/tracker/jira/
cp internal/jira/client_docs.go internal/tracker/jira/
cp internal/jira/jql.go internal/tracker/jira/
cp internal/jira/*_test.go internal/tracker/jira/
```

- [ ] **Step 2: Обновить package declaration во всех файлах**

Заменить `package jira` на `package jira` (остаётся тем же — пакет называется `jira`, но путь другой). Добавить import `"github.com/grevus/mcp-jira/internal/tracker"`.

В каждом файле заменить возвращаемые типы:
- `Issue` → `tracker.Issue` (в публичных методах)
- `ListIssuesParams` → `tracker.ListParams`
- `SprintHealth` → `tracker.SprintHealth`
- `SprintReport` → `tracker.SprintReport`
- `IssueDoc` → `tracker.IssueDoc`

Приватные DTO (`searchResponse`, `sprintIssue`, `docsIssueResponse` и т.д.) оставить как есть — это деталь реализации Jira-клиента.

Ключевые изменения в `client_issues.go`:

```go
import (
	"github.com/grevus/mcp-jira/internal/tracker"
)

func (c *HTTPClient) GetIssue(ctx context.Context, key string) (tracker.Issue, string, error) {
	// ... парсинг ...
	return tracker.Issue{
		Key:      ir.Key,
		Summary:  ir.Fields.Summary,
		Status:   ir.Fields.Status.Name,
		Assignee: assignee,
	}, ir.Fields.Description, nil
}

func (c *HTTPClient) ListIssues(ctx context.Context, p tracker.ListParams) ([]tracker.Issue, error) {
	// ... сохраняем всю логику JQL, заменяем только входные/выходные типы ...
	issues := make([]tracker.Issue, 0, len(sr.Issues))
	for _, ir := range sr.Issues {
		// ... конвертация ...
		issues = append(issues, tracker.Issue{...})
	}
	return issues, nil
}
```

Аналогичные замены в `client_sprints.go` (SprintHealth → tracker.SprintHealth и т.д.) и `client_docs.go` (IssueDoc → tracker.IssueDoc).

- [ ] **Step 3: Проверить компиляцию нового пакета**

Run: `go build ./internal/tracker/jira/`
Expected: SUCCESS

- [ ] **Step 4: Запустить тесты нового пакета**

Run: `go test ./internal/tracker/jira/ -v`
Expected: все тесты проходят

- [ ] **Step 5: Commit**

```bash
git add internal/tracker/jira/
git commit -m "feat: move jira client to internal/tracker/jira with tracker types"
```

---

### Task 3: Создать `internal/knowledge/knowledge.go` — domain types + интерфейсы

**Files:**
- Create: `internal/knowledge/knowledge.go`

- [ ] **Step 1: Создать файл**

```go
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
```

- [ ] **Step 2: Проверить компиляцию**

Run: `go build ./internal/knowledge/`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/knowledge/knowledge.go
git commit -m "feat: add internal/knowledge package with tracker-agnostic types"
```

---

### Task 4: Перенести `internal/rag/embed/` → `internal/knowledge/embed/`

**Files:**
- Create: `internal/knowledge/embed/` (копия `internal/rag/embed/`)

Без изменений — embedder не зависит от Jira-типов.

- [ ] **Step 1: Скопировать файлы**

```bash
mkdir -p internal/knowledge/embed
cp internal/rag/embed/*.go internal/knowledge/embed/
```

- [ ] **Step 2: Проверить компиляцию**

Run: `go build ./internal/knowledge/embed/`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/knowledge/embed/
git commit -m "refactor: move embed package to internal/knowledge/embed"
```

---

### Task 5: Перенести `internal/rag/store/` → `internal/knowledge/pgvector/` + миграция multi-tenant

**Files:**
- Create: `internal/knowledge/pgvector/store.go`, `query.go`, `upsert.go`, `replace.go`, `stats.go`, `migrations.go`
- Create: `internal/knowledge/pgvector/migrations/001_init.sql` (без изменений)
- Create: `internal/knowledge/pgvector/migrations/002_multi_tenant.sql`
- Copy tests: все `*_test.go`

- [ ] **Step 1: Скопировать файлы и обновить package**

```bash
mkdir -p internal/knowledge/pgvector/migrations
cp internal/rag/store/postgres.go internal/knowledge/pgvector/store.go
cp internal/rag/store/postgres_query.go internal/knowledge/pgvector/query.go
cp internal/rag/store/postgres_upsert.go internal/knowledge/pgvector/upsert.go
cp internal/rag/store/postgres_replace.go internal/knowledge/pgvector/replace.go
cp internal/rag/store/postgres_stats.go internal/knowledge/pgvector/stats.go
cp internal/rag/store/migrations.go internal/knowledge/pgvector/migrations.go
cp internal/rag/store/migrations/001_init.sql internal/knowledge/pgvector/migrations/
cp internal/rag/store/*_test.go internal/knowledge/pgvector/
```

Заменить `package store` на `package pgvector` во всех файлах.

- [ ] **Step 2: Обновить типы на knowledge.Document/Hit/Filter**

В `store.go`: заменить package declaration на `pgvector`, тип остаётся `PgvectorStore`.

В `query.go`: обновить SQL и типы:

```go
package pgvector

import (
	"context"
	"fmt"

	"github.com/grevus/mcp-jira/internal/knowledge"
	pgvec "github.com/pgvector/pgvector-go"
)

const querySQL = `
SELECT doc_key, title, status,
       substring(content, 1, 300) AS excerpt,
       1 - (embedding <=> $1)     AS score
FROM issues_index
WHERE project_key = $2
  AND ($3 = '' OR tenant_id = $3)
  AND ($4 = '' OR source = $4)
ORDER BY embedding <=> $1
LIMIT $5`

func (s *PgvectorStore) Search(ctx context.Context, queryEmbedding []float32, f knowledge.Filter, topK int) ([]knowledge.Hit, error) {
	if topK == 0 || len(queryEmbedding) == 0 {
		return []knowledge.Hit{}, nil
	}

	rows, err := s.pool.Query(ctx, querySQL,
		pgvec.NewVector(queryEmbedding),
		f.ProjectKey,
		f.TenantID,
		f.Source,
		topK,
	)
	if err != nil {
		return nil, fmt.Errorf("pgvector: Search: %w", err)
	}
	defer rows.Close()

	var hits []knowledge.Hit
	for rows.Next() {
		var h knowledge.Hit
		if err := rows.Scan(&h.DocKey, &h.Title, &h.Status, &h.Excerpt, &h.Score); err != nil {
			return nil, fmt.Errorf("pgvector: Search: scan: %w", err)
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgvector: Search: rows: %w", err)
	}
	if hits == nil {
		hits = []knowledge.Hit{}
	}
	return hits, nil
}
```

В `upsert.go`:

```go
const upsertSQL = `
INSERT INTO issues_index (tenant_id, source, project_key, doc_key, title, status, author, content, embedding, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (tenant_id, project_key, doc_key) DO UPDATE SET
    source      = EXCLUDED.source,
    title       = EXCLUDED.title,
    status      = EXCLUDED.status,
    author      = EXCLUDED.author,
    content     = EXCLUDED.content,
    embedding   = EXCLUDED.embedding,
    updated_at  = EXCLUDED.updated_at`

func (s *PgvectorStore) Upsert(ctx context.Context, docs []knowledge.Document) error {
	if len(docs) == 0 {
		return nil
	}
	for _, doc := range docs {
		_, err := s.pool.Exec(ctx, upsertSQL,
			doc.TenantID, doc.Source, doc.ProjectKey, doc.DocKey,
			doc.Title, doc.Status, doc.Author, doc.Content,
			pgvec.NewVector(doc.Embedding), doc.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("pgvector: Upsert: %w", err)
		}
	}
	return nil
}
```

В `replace.go`:

```go
const deleteProjectSQL = `DELETE FROM issues_index WHERE tenant_id = $1 AND project_key = $2`

func (s *PgvectorStore) ReplaceProject(ctx context.Context, tenantID, projectKey string, docs []knowledge.Document) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pgvector: ReplaceProject: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, deleteProjectSQL, tenantID, projectKey); err != nil {
		return fmt.Errorf("pgvector: ReplaceProject: delete: %w", err)
	}

	for _, doc := range docs {
		_, err = tx.Exec(ctx, upsertSQL,
			doc.TenantID, doc.Source, doc.ProjectKey, doc.DocKey,
			doc.Title, doc.Status, doc.Author, doc.Content,
			pgvec.NewVector(doc.Embedding), doc.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("pgvector: ReplaceProject: insert %s: %w", doc.DocKey, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("pgvector: ReplaceProject: commit: %w", err)
	}
	return nil
}
```

В `stats.go`:

```go
func (s *PgvectorStore) Stats(ctx context.Context, tenantID, projectKey string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		"SELECT count(*) FROM issues_index WHERE tenant_id=$1 AND project_key=$2",
		tenantID, projectKey,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("pgvector: Stats: %w", err)
	}
	return count, nil
}
```

- [ ] **Step 3: Создать миграцию 002_multi_tenant.sql**

```sql
-- +goose Up
ALTER TABLE issues_index ADD COLUMN tenant_id TEXT NOT NULL DEFAULT '';
ALTER TABLE issues_index ADD COLUMN source TEXT NOT NULL DEFAULT 'jira';
ALTER TABLE issues_index RENAME COLUMN issue_key TO doc_key;
ALTER TABLE issues_index RENAME COLUMN summary TO title;
ALTER TABLE issues_index RENAME COLUMN assignee TO author;

ALTER TABLE issues_index DROP CONSTRAINT IF EXISTS issues_index_issue_key_key;
ALTER TABLE issues_index ADD CONSTRAINT issues_index_tenant_project_doc_key
    UNIQUE (tenant_id, project_key, doc_key);

-- +goose Down
ALTER TABLE issues_index DROP CONSTRAINT IF EXISTS issues_index_tenant_project_doc_key;
ALTER TABLE issues_index RENAME COLUMN doc_key TO issue_key;
ALTER TABLE issues_index RENAME COLUMN title TO summary;
ALTER TABLE issues_index RENAME COLUMN author TO assignee;
ALTER TABLE issues_index DROP COLUMN IF EXISTS source;
ALTER TABLE issues_index DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE issues_index ADD CONSTRAINT issues_index_issue_key_key UNIQUE (issue_key);
```

- [ ] **Step 4: Проверить компиляцию**

Run: `go build ./internal/knowledge/pgvector/`
Expected: SUCCESS

- [ ] **Step 5: Commit**

```bash
git add internal/knowledge/pgvector/
git commit -m "feat: move rag/store to knowledge/pgvector with multi-tenant support"
```

---

### Task 6: Перенести `internal/rag/index/` → `internal/knowledge/index/`

**Files:**
- Create: `internal/knowledge/index/indexer.go`, `render.go`
- Copy tests: `*_test.go`

- [ ] **Step 1: Скопировать и обновить импорты**

```bash
mkdir -p internal/knowledge/index
cp internal/rag/index/*.go internal/knowledge/index/
```

В `indexer.go`: заменить `jira.IssueDoc` → `tracker.IssueDoc`, `store.Document` → `knowledge.Document`. Обновить imports:

```go
import (
	"github.com/grevus/mcp-jira/internal/tracker"
	"github.com/grevus/mcp-jira/internal/knowledge"
)

type IssueReader interface {
	IterateIssueDocs(ctx context.Context, projectKey string) (<-chan tracker.IssueDoc, <-chan error)
}

type Store interface {
	ReplaceProject(ctx context.Context, tenantID, projectKey string, docs []knowledge.Document) error
}
```

В `Reindex` добавить параметры `tenantID` и `source`:

```go
func (idx *Indexer) Reindex(ctx context.Context, tenantID, source, projectKey string) (int, error) {
	// ... существующая логика ...
	// При создании Document:
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
	// ...
	if err := idx.Store.ReplaceProject(ctx, tenantID, projectKey, documents); err != nil {
	// ...
}
```

В `render.go`: заменить `jira.IssueDoc` → `tracker.IssueDoc`:

```go
import "github.com/grevus/mcp-jira/internal/tracker"

func RenderDoc(d tracker.IssueDoc) string {
	// ... без изменений логики ...
}
```

- [ ] **Step 2: Обновить тесты**

Заменить `jira.IssueDoc` → `tracker.IssueDoc` в тестах.

- [ ] **Step 3: Проверить компиляцию и тесты**

Run: `go build ./internal/knowledge/index/ && go test ./internal/knowledge/index/ -v`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/knowledge/index/
git commit -m "refactor: move rag/index to knowledge/index with tracker types"
```

---

### Task 7: Перенести `internal/rag/retriever/` → `internal/knowledge/retriever/`

**Files:**
- Create: `internal/knowledge/retriever/retriever.go`
- Copy tests: `*_test.go`

- [ ] **Step 1: Скопировать и обновить**

```bash
mkdir -p internal/knowledge/retriever
cp internal/rag/retriever/*.go internal/knowledge/retriever/
```

Обновить импорты и типы:

```go
package retriever

import (
	"context"
	"fmt"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type Store interface {
	Search(ctx context.Context, queryEmbedding []float32, f knowledge.Filter, topK int) ([]knowledge.Hit, error)
}

type Retriever struct {
	Embedder Embedder
	Store    Store
	TenantID string // подставляется в Filter автоматически
}

func New(e Embedder, s Store, tenantID string) *Retriever {
	return &Retriever{Embedder: e, Store: s, TenantID: tenantID}
}

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
```

- [ ] **Step 2: Обновить тесты**

- [ ] **Step 3: Проверить компиляцию и тесты**

Run: `go build ./internal/knowledge/retriever/ && go test ./internal/knowledge/retriever/ -v`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/knowledge/retriever/
git commit -m "refactor: move rag/retriever to knowledge/retriever with tenant support"
```

---

### Task 8: Обновить `internal/handlers/` — `jira.*` → `tracker.*`

**Files:**
- Modify: все файлы в `internal/handlers/`

Это механическая замена во всех handler-файлах.

- [ ] **Step 1: Обновить `handlers/knowledge.go`**

Заменить `handlers.Hit` на `knowledge.Hit`:

```go
package handlers

import (
	"context"
	"fmt"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

// KnowledgeRetriever — узкий интерфейс для handler SearchKnowledge.
type KnowledgeRetriever interface {
	Search(ctx context.Context, projectKey, query string, topK int) ([]knowledge.Hit, error)
}

type SearchKnowledgeOutput struct {
	Hits []knowledge.Hit `json:"hits"`
}
```

Удалить старый тип `Hit` из `knowledge.go`.

- [ ] **Step 2: Обновить `handlers/issues.go`**

Заменить `jira.ListIssuesParams` → `tracker.ListParams`, `jira.Issue` → `tracker.Issue`:

```go
import "github.com/grevus/mcp-jira/internal/tracker"

type IssueLister interface {
	ListIssues(ctx context.Context, p tracker.ListParams) ([]tracker.Issue, error)
}

type ListIssuesOutput struct {
	Issues []tracker.Issue `json:"issues"`
}

func ListIssues(l IssueLister) Handler[ListIssuesInput, ListIssuesOutput] {
	return func(ctx context.Context, in ListIssuesInput) (ListIssuesOutput, error) {
		issues, err := l.ListIssues(ctx, tracker.ListParams{
			ProjectKey: in.ProjectKey,
			Status:     in.Status,
			Assignee:   in.AssignedTo,
			Limit:      in.Limit,
		})
		// ...
	}
}
```

- [ ] **Step 3: Обновить `handlers/sprints.go`**

```go
import "github.com/grevus/mcp-jira/internal/tracker"

type SprintReader interface {
	GetSprintHealth(ctx context.Context, boardID int) (tracker.SprintHealth, error)
}

type SprintHealthOutput struct {
	Health tracker.SprintHealth `json:"health"`
}
```

- [ ] **Step 4: Обновить `handlers/sprint_report.go`**

Заменить `jira.SprintReport` → `tracker.SprintReport`, `jira.SprintHealth` → `tracker.SprintHealth`:

```go
import "github.com/grevus/mcp-jira/internal/tracker"

type SprintReporter interface {
	GetSprintReport(ctx context.Context, boardID, sprintID int) (tracker.SprintReport, error)
}

type SprintHealthReportOutput struct {
	Report       tracker.SprintReport `json:"report"`
	// ...
}

func computeRisk(h tracker.SprintHealth) RiskLevel { ... }
```

- [ ] **Step 5: Обновить `handlers/similar_issues.go`**

```go
import "github.com/grevus/mcp-jira/internal/tracker"

type IssueFetcher interface {
	GetIssue(ctx context.Context, key string) (tracker.Issue, string, error)
}

type SimilarIssuesOutput struct {
	Source        tracker.Issue     `json:"source"`
	SimilarIssues []knowledge.Hit  `json:"similar_issues"`
}
```

Заменить `h.IssueKey` → `h.DocKey` в фильтрации self-match.

- [ ] **Step 6: Обновить `handlers/standup_digest.go`**

```go
import "github.com/grevus/mcp-jira/internal/tracker"

type StandupDigestOutput struct {
	// ...
	Blockers []tracker.Issue `json:"blockers"`
	// ...
}

func StandupDigest(l IssueLister) Handler[...] {
	return func(ctx context.Context, in StandupDigestInput) (StandupDigestOutput, error) {
		issues, err := l.ListIssues(ctx, tracker.ListParams{
			ProjectKey:  in.TeamKey,
			UpdatedFrom: in.From,
			UpdatedTo:   in.To,
			Limit:       in.Limit,
		})
		// ...
	}
}
```

- [ ] **Step 7: Обновить `handlers/incident_context.go`**

```go
import (
	"github.com/grevus/mcp-jira/internal/tracker"
	"github.com/grevus/mcp-jira/internal/knowledge"
)

type IncidentContextOutput struct {
	Source            tracker.Issue    `json:"source"`
	RelatedIncidents  []knowledge.Hit `json:"related_incidents"`
	// ...
}
```

Заменить `h.IssueKey` → `h.DocKey` в фильтрации self-match.

- [ ] **Step 8: Обновить `handlers/ticket_triage.go`**

```go
type TicketTriageOutput struct {
	Source         tracker.Issue    `json:"source"`
	// ...
	SimilarIssues  []knowledge.Hit `json:"similar_issues"`
}
```

Заменить `h.IssueKey` → `h.DocKey`.

- [ ] **Step 9: Обновить `handlers/release_risk_check.go`**

```go
type ReleaseRiskCheckOutput struct {
	// ...
	OpenIssues         []tracker.Issue  `json:"open_issues"`
	BlockedIssues      []tracker.Issue  `json:"blocked_issues"`
	RelatedPostmortems []knowledge.Hit  `json:"related_postmortems"`
	// ...
}

func ReleaseRiskCheck(l IssueLister, r KnowledgeRetriever) Handler[...] {
	return func(ctx context.Context, in ReleaseRiskCheckInput) (ReleaseRiskCheckOutput, error) {
		issues, err := l.ListIssues(ctx, tracker.ListParams{
			ProjectKey: in.ProjectKey,
			FixVersion: in.FixVersion,
			Limit:      100,
		})
		// ...
		open := make([]tracker.Issue, 0)
		blocked := make([]tracker.Issue, 0)
		// ...
	}
}
```

- [ ] **Step 10: Обновить все handler тесты**

Заменить `jira.Issue{}` → `tracker.Issue{}`, `jira.ListIssuesParams` → `tracker.ListParams` и т.д. во всех `*_test.go`. Заменить `handlers.Hit` → `knowledge.Hit`. Заменить поле `IssueKey` → `DocKey` в Hit.

- [ ] **Step 11: Проверить компиляцию и тесты**

Run: `go build ./internal/handlers/ && go test ./internal/handlers/ -v`
Expected: SUCCESS, все тесты проходят

- [ ] **Step 12: Commit**

```bash
git add internal/handlers/
git commit -m "refactor: switch handlers from jira types to tracker/knowledge types"
```

---

### Task 9: Создать `internal/tenant/` — Registry + loader

**Files:**
- Create: `internal/tenant/tenant.go`, `internal/tenant/loader.go`
- Test: `internal/tenant/tenant_test.go`, `internal/tenant/loader_test.go`

- [ ] **Step 1: Написать тест для loader**

```go
package tenant_test

import (
	"os"
	"testing"

	"github.com/grevus/mcp-jira/internal/tenant"
	"github.com/stretchr/testify/require"
)

func TestLoadTenantsFromFile_Full(t *testing.T) {
	content := `keys:
  - key: "sk-alice"
    name: "Alice"
    tracker: jira
    tracker_config:
      base_url: "https://alice.atlassian.net"
      email: "a@a.com"
      api_token: "tok"
      auth_type: "basic"
    projects: ["PROJ"]
  - key: "sk-bob"
    name: "Bob"
    tracker: jira
    tracker_config:
      base_url: "https://bob.atlassian.net"
      email: "b@b.com"
      api_token: "tok2"
    projects: ["BACK"]
`
	path := t.TempDir() + "/keys.yaml"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	configs, err := tenant.LoadTenantsFromFile(path)
	require.NoError(t, err)
	require.Len(t, configs, 2)
	require.Equal(t, "Alice", configs[0].Name)
	require.Equal(t, "jira", configs[0].TrackerType)
	require.Equal(t, "https://alice.atlassian.net", configs[0].TrackerConfig["base_url"])
	require.Equal(t, []string{"PROJ"}, configs[0].ProjectKeys)
}

func TestLoadTenantsFromFile_Legacy(t *testing.T) {
	content := `keys:
  - key: "sk-alice"
    name: "Alice"
`
	path := t.TempDir() + "/keys.yaml"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	configs, err := tenant.LoadTenantsFromFile(path)
	require.NoError(t, err)
	require.Len(t, configs, 1)
	require.Equal(t, "", configs[0].TrackerType)
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `go test ./internal/tenant/ -v`
Expected: FAIL — package not found

- [ ] **Step 3: Создать `tenant.go`**

```go
package tenant

import (
	"fmt"

	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/tracker"
)

// Config — конфигурация одного тенанта.
type Config struct {
	APIKey        string
	Name          string
	TrackerType   string
	TrackerConfig map[string]string
	ProjectKeys   []string
}

// Tenant — runtime-состояние клиента.
type Tenant struct {
	Config    Config
	Provider  tracker.Provider
	Knowledge knowledge.Store
}

// Registry хранит тенантов и их провайдеров.
type Registry struct {
	tenants map[string]*Tenant // key name → tenant
}

// NewRegistry создаёт пустой Registry.
func NewRegistry() *Registry {
	return &Registry{tenants: make(map[string]*Tenant)}
}

// Register добавляет тенанта в реестр.
func (r *Registry) Register(name string, t *Tenant) {
	r.tenants[name] = t
}

// Resolve возвращает тенанта по имени ключа.
func (r *Registry) Resolve(keyName string) (*Tenant, error) {
	t, ok := r.tenants[keyName]
	if !ok {
		return nil, fmt.Errorf("tenant: unknown key %q", keyName)
	}
	return t, nil
}
```

- [ ] **Step 4: Создать `loader.go`**

```go
package tenant

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type keyEntry struct {
	Key           string            `yaml:"key"`
	Name          string            `yaml:"name"`
	TrackerType   string            `yaml:"tracker"`
	TrackerConfig map[string]string `yaml:"tracker_config"`
	Projects      []string          `yaml:"projects"`
}

type keysFile struct {
	Keys []keyEntry `yaml:"keys"`
}

// LoadTenantsFromFile читает расширенный keys.yaml и возвращает список Config.
func LoadTenantsFromFile(path string) ([]Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tenant: read file: %w", err)
	}

	var f keysFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("tenant: parse file: %w", err)
	}

	if len(f.Keys) == 0 {
		return nil, fmt.Errorf("tenant: no keys in %s", path)
	}

	configs := make([]Config, 0, len(f.Keys))
	for i, k := range f.Keys {
		if k.Key == "" {
			return nil, fmt.Errorf("tenant: key #%d has empty value", i+1)
		}
		if k.Name == "" {
			k.Name = fmt.Sprintf("key-%d", i+1)
		}
		configs = append(configs, Config{
			APIKey:        k.Key,
			Name:          k.Name,
			TrackerType:   k.TrackerType,
			TrackerConfig: k.TrackerConfig,
			ProjectKeys:   k.Projects,
		})
	}

	return configs, nil
}
```

- [ ] **Step 5: Запустить тесты**

Run: `go test ./internal/tenant/ -v`
Expected: PASS

- [ ] **Step 6: Написать тест для Registry**

```go
func TestRegistry_Resolve(t *testing.T) {
	reg := tenant.NewRegistry()
	ten := &tenant.Tenant{Config: tenant.Config{Name: "Alice"}}
	reg.Register("Alice", ten)

	got, err := reg.Resolve("Alice")
	require.NoError(t, err)
	require.Equal(t, "Alice", got.Config.Name)

	_, err = reg.Resolve("unknown")
	require.Error(t, err)
}
```

- [ ] **Step 7: Запустить все тесты tenant**

Run: `go test ./internal/tenant/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/tenant/
git commit -m "feat: add internal/tenant package with Registry and YAML loader"
```

---

### Task 10: Обновить `internal/register/` — `adaptTenant`

**Files:**
- Modify: `internal/register/register.go`
- Modify: `internal/register/adapt.go`

- [ ] **Step 1: Добавить `adaptTenant` в `adapt.go`**

```go
import (
	"github.com/grevus/mcp-jira/internal/auth"
	"github.com/grevus/mcp-jira/internal/tenant"
)

// adaptTenant создаёт MCP handler, который резолвит тенанта из context
// и делегирует в handler, созданный factory.
func adaptTenant[In, Out any](
	reg *tenant.Registry,
	factory func(t *tenant.Tenant) handlers.Handler[In, Out],
) func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		keyName := auth.KeyNameFromContext(ctx)
		t, err := reg.Resolve(keyName)
		if err != nil {
			var zero Out
			return nil, zero, err
		}
		h := factory(t)
		return adapt(h)(ctx, req, in)
	}
}
```

- [ ] **Step 2: Обновить `register.go`**

```go
package register

import (
	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/tenant"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register регистрирует все MCP-инструменты в srv с multi-tenant резолвингом.
func Register(srv *mcp.Server, reg *tenant.Registry) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_issues",
		Description: "Search issues using filters (project, status, assignee).",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.ListIssuesInput, handlers.ListIssuesOutput] {
		return handlers.ListIssues(t.Provider)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_sprint_health",
		Description: "Return health metrics for the active sprint of a board.",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.SprintHealthInput, handlers.SprintHealthOutput] {
		return handlers.SprintHealth(t.Provider)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_jira_knowledge",
		Description: "Semantic search over indexed issues for a given project. Use for free-text questions when you don't have a specific issue key. Not a substitute for live filters — data is as fresh as the last indexer run.",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.SearchKnowledgeInput, handlers.SearchKnowledgeOutput] {
		ret := newRetrieverForTenant(t)
		return handlers.SearchKnowledge(ret)
	}))

	// ... аналогично для остальных 7 tools ...
}
```

Примечание: `newRetrieverForTenant(t)` — хелпер, который создаёт retriever с правильным `TenantID`. Определяется в `register.go`:

```go
import "github.com/grevus/mcp-jira/internal/knowledge/retriever"

func newRetrieverForTenant(t *tenant.Tenant) *retriever.Retriever {
	// Embedder берём из tenant или из глобального — зависит от wiring в main.go
	// На данном этапе Embedder глобальный, передаётся через замыкание
	return nil // placeholder — будет заполнен при wiring в main.go
}
```

Фактически retriever будет создаваться в `main.go` и храниться в `Tenant`. Handlers получат его через `KnowledgeRetriever` интерфейс.

Обновлённая сигнатура `Register`:

```go
func Register(srv *mcp.Server, reg *tenant.Registry) {
	// ... все tools с adaptTenant ...
}
```

- [ ] **Step 3: Проверить компиляцию**

Run: `go build ./internal/register/`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/register/
git commit -m "feat: add adaptTenant for multi-tenant tool resolution"
```

---

### Task 11: Обновить `cmd/server/main.go` — Registry wiring

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Обновить main.go**

Ключевые изменения:
- Создать `tenant.Registry`
- Для каждого тенанта из config создать `tracker.Provider` (jira клиент) и `Retriever`
- Передать `Registry` в `register.Register`
- Обратная совместимость: если `MCP_KEYS_FILE` нет — single-tenant из env

```go
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/labstack/echo/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/grevus/mcp-jira/internal/auth"
	"github.com/grevus/mcp-jira/internal/config"
	"github.com/grevus/mcp-jira/internal/knowledge/embed"
	"github.com/grevus/mcp-jira/internal/knowledge/pgvector"
	"github.com/grevus/mcp-jira/internal/knowledge/retriever"
	"github.com/grevus/mcp-jira/internal/register"
	"github.com/grevus/mcp-jira/internal/tenant"
	jiratracker "github.com/grevus/mcp-jira/internal/tracker/jira"
)

func main() {
	// ... flag parsing, config loading, embedder creation, store creation ...

	reg := tenant.NewRegistry()

	if cfg.MCPKeysFile != "" {
		tenantConfigs, err := tenant.LoadTenantsFromFile(cfg.MCPKeysFile)
		if err != nil {
			log.Fatalf("tenants: %v", err)
		}
		for _, tc := range tenantConfigs {
			var provider tracker.Provider
			switch tc.TrackerType {
			case "jira", "":
				provider = jiratracker.NewHTTPClient(
					tc.TrackerConfig["base_url"],
					tc.TrackerConfig["email"],
					tc.TrackerConfig["api_token"],
					tc.TrackerConfig["auth_type"],
					nil,
				)
			default:
				log.Fatalf("unknown tracker type %q for tenant %q", tc.TrackerType, tc.Name)
			}
			ret := retriever.New(emb, st, tc.Name)
			ten := &tenant.Tenant{
				Config:    tc,
				Provider:  provider,
				Knowledge: st,
				Retriever: ret,
			}
			reg.Register(tc.Name, ten)
		}
	} else {
		// single-tenant fallback
		jc := jiratracker.NewHTTPClient(cfg.JiraBaseURL, cfg.JiraEmail, cfg.JiraAPIToken, cfg.JiraAuthType, nil)
		ret := retriever.New(emb, st, "")
		ten := &tenant.Tenant{
			Config:    tenant.Config{Name: "default"},
			Provider:  jc,
			Knowledge: st,
			Retriever: ret,
		}
		reg.Register("default", ten)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "mcp-jira", Version: "0.2.0"}, nil)
	register.Register(srv, reg)

	// ... transport setup ...
}
```

- [ ] **Step 2: Добавить `Retriever` поле в `tenant.Tenant`**

В `internal/tenant/tenant.go` добавить поле для retriever:

```go
type Tenant struct {
	Config    Config
	Provider  tracker.Provider
	Knowledge knowledge.Store
	Retriever interface {
		Search(ctx context.Context, projectKey, query string, topK int) ([]knowledge.Hit, error)
	}
}
```

- [ ] **Step 3: Обновить `register.go` для использования `t.Retriever`**

Вместо `newRetrieverForTenant(t)` использовать `t.Retriever` напрямую как `KnowledgeRetriever`.

- [ ] **Step 4: Проверить компиляцию**

Run: `go build ./cmd/server`
Expected: SUCCESS

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go internal/tenant/tenant.go internal/register/register.go
git commit -m "feat: wire multi-tenant Registry in server main"
```

---

### Task 12: Обновить `cmd/index/main.go` — `--tenant` flag

**Files:**
- Modify: `cmd/index/main.go`

- [ ] **Step 1: Добавить `--tenant` flag**

```go
func runIndex(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	projectKey := fs.String("project", "", "Project key to reindex (required)")
	tenantName := fs.String("tenant", "", "Tenant name from keys.yaml (optional)")
	keysFile := fs.String("keys-file", "", "Path to keys.yaml (required with --tenant)")
	// ...

	var tenantID, source string
	if *tenantName != "" {
		if *keysFile == "" {
			log.Fatal("index: --keys-file is required with --tenant")
		}
		configs, err := tenant.LoadTenantsFromFile(*keysFile)
		// find matching tenant config, create jira client from it
		// tenantID = tenantName
		source = tc.TrackerType
		jc = jiratracker.NewHTTPClient(...)
	} else {
		// legacy: from env
		tenantID = ""
		source = "jira"
		jc = jiratracker.NewHTTPClient(cfg.JiraBaseURL, cfg.JiraEmail, cfg.JiraAPIToken, cfg.JiraAuthType, nil)
	}

	indexer := kindex.New(jc, emb, st)
	n, err := indexer.Reindex(ctx, tenantID, source, *projectKey)
}
```

- [ ] **Step 2: Обновить imports**

Заменить `internal/jira` → `internal/tracker/jira`, `internal/rag/*` → `internal/knowledge/*`.

- [ ] **Step 3: Обновить `runMigrate`**

```go
import kpg "github.com/grevus/mcp-jira/internal/knowledge/pgvector"

func runMigrate(ctx context.Context) {
	// ...
	if err := kpg.Migrate(ctx, db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
}
```

- [ ] **Step 4: Проверить компиляцию**

Run: `go build ./cmd/index`
Expected: SUCCESS

- [ ] **Step 5: Commit**

```bash
git add cmd/index/main.go
git commit -m "feat: add --tenant flag to index CLI"
```

---

### Task 13: Удалить старые пакеты `internal/jira/` и `internal/rag/`

**Files:**
- Delete: `internal/jira/` (весь пакет)
- Delete: `internal/rag/` (весь пакет)

- [ ] **Step 1: Убедиться, что ничего не импортирует старые пакеты**

Run: `grep -r '"github.com/grevus/mcp-jira/internal/jira"' --include='*.go' . | grep -v '_test.go' | grep -v 'internal/jira/'`
Expected: no matches

Run: `grep -r '"github.com/grevus/mcp-jira/internal/rag' --include='*.go' . | grep -v '_test.go' | grep -v 'internal/rag/'`
Expected: no matches

- [ ] **Step 2: Удалить старые пакеты**

```bash
rm -rf internal/jira internal/rag
```

- [ ] **Step 3: go mod tidy + полная сборка + тесты**

```bash
go mod tidy
go build ./...
go test ./...
```
Expected: BUILD SUCCESS, все тесты проходят

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor: remove old internal/jira and internal/rag packages"
```

---

### Task 14: Обновить `internal/config/config.go` — backward-compatible Jira env

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Сделать Jira env опциональными при наличии `MCP_KEYS_FILE` с tracker**

В `Load()` для `ModeHTTP`: если задан `MCP_KEYS_FILE`, не требовать `JIRA_BASE_URL`, `JIRA_API_TOKEN` — они берутся из YAML per-tenant.

```go
if mode == ModeHTTP && mcpKeysFile != "" {
	// В multi-tenant режиме Jira env не обязательны
	// Они нужны только как fallback для записей без tracker_config
} else {
	// Проверяем обязательные Jira env как раньше
}
```

- [ ] **Step 2: Проверить компиляцию и тесты**

Run: `go build ./... && go test ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "fix: make Jira env optional when MCP_KEYS_FILE with tracker_config is used"
```

---

### Task 15: Финальная проверка — полная сборка и тесты

**Files:** все

- [ ] **Step 1: go mod tidy**

Run: `go mod tidy`

- [ ] **Step 2: Полная сборка**

Run: `go build ./cmd/server && go build ./cmd/index`
Expected: SUCCESS

- [ ] **Step 3: Все юнит-тесты**

Run: `go test ./...`
Expected: все тесты проходят

- [ ] **Step 4: go vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 5: Commit финальный**

```bash
git add -A
git commit -m "chore: tracker abstraction refactor complete — all tests green"
```

---

## Порядок выполнения и зависимости

```
Task 1 (tracker types)
  ↓
Task 2 (jira → tracker/jira)
  ↓
Task 3 (knowledge types) ─── Task 4 (embed copy)
  ↓                            ↓
Task 5 (pgvector + migration)
  ↓
Task 6 (knowledge/index)
  ↓
Task 7 (knowledge/retriever)
  ↓
Task 8 (handlers update) ←── зависит от 1,3,7
  ↓
Task 9 (tenant package)
  ↓
Task 10 (register update) ←── зависит от 8,9
  ↓
Task 11 (server main) ←── зависит от 2,7,9,10
  ↓
Task 12 (index main) ←── зависит от 2,6,9
  ↓
Task 13 (delete old packages) ←── зависит от 11,12
  ↓
Task 14 (config update)
  ↓
Task 15 (final verification)
```
