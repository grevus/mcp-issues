# Tracker Abstraction & Knowledge Base — Design Spec

**Дата:** 2026-04-13
**Статус:** Approved
**Предыдущий spec:** `2026-04-06-mcp-jira-design.md`

## 1. Цель

Ввести два tracker-agnostic интерфейса, чтобы MCP-сервер мог работать с любым трекером задач (Jira, YouTrack, Linear, ...) и любым источником знаний. Мотивация — продажа клиентам с разными трекерами: один MCP-сервер, разные бэкенды.

## 2. Scope

### В scope

- Пакет `internal/tracker/` — domain types + интерфейсы + реализация Jira
- Пакет `internal/knowledge/` — domain types + интерфейсы + pgvector-реализация
- Пакет `internal/tenant/` — Registry + загрузка конфига из YAML
- Миграция pgvector: `tenant_id`, `source`, `doc_key`
- Обновление `internal/handlers/` — замена `jira.*` на `tracker.*`
- Обновление `internal/register/` — `adaptTenant` для multi-tenant резолвинга
- Обновление `cmd/server/` и `cmd/index/`
- Обратная совместимость: single-tenant режим через env

### Не в scope (отдельные планы)

- Реализация YouTrack/Linear провайдера — только интерфейс
- Хранение credentials в БД — сейчас YAML, потом отдельный план
- Per-tenant rate limiting
- Per-tenant usage tracking / billing
- Confluence как источник знаний (Phase 3 — отдельный spec)
- Hot-reload tenant config
- Тесты конкретных трекеров кроме Jira

## 3. Архитектура

### 3.1. Интерфейс трекера — `internal/tracker/`

Пакет определяет domain types и узкие интерфейсы. Ничего не знает про конкретные трекеры, pgvector, MCP.

#### Domain types

```go
package tracker

import "time"

type Issue struct {
    Key      string
    Summary  string
    Status   string
    Assignee string
}

type ListParams struct {
    ProjectKey  string
    Status      string
    Assignee    string
    FixVersion  string
    UpdatedFrom string // "YYYY-MM-DD" или "YYYY-MM-DD HH:MM"
    UpdatedTo   string
    Limit       int
}

type SprintHealth struct {
    BoardID    int
    SprintName string
    Total      int
    Done       int
    InProgress int
    Blocked    int
    Velocity   float64
}

type SprintReport struct {
    Health        SprintHealth
    BlockedIssues []Issue
    ScopeAdded    []Issue
    ScopeRemoved  []Issue
}

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
```

#### Узкие интерфейсы

```go
type IssueLister interface {
    ListIssues(ctx context.Context, p ListParams) ([]Issue, error)
}

type IssueFetcher interface {
    GetIssue(ctx context.Context, key string) (Issue, string, error)
}

type SprintReader interface {
    GetSprintHealth(ctx context.Context, boardID int) (SprintHealth, error)
}

type SprintReporter interface {
    GetSprintReport(ctx context.Context, boardID, sprintID int) (SprintReport, error)
}

type ScopeReader interface {
    GetSprintScopeChanges(ctx context.Context, sprintID int) (added, removed []string, err error)
}

type CommentFetcher interface {
    GetIssueComments(ctx context.Context, issueKey string) ([]string, error)
}

type DocIterator interface {
    IterateIssueDocs(ctx context.Context, projectKey string) (<-chan IssueDoc, <-chan error)
}
```

#### Provider — суперинтерфейс для wiring

```go
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

Handlers продолжают принимать узкие интерфейсы (ISP). `Provider` используется только в `register` и `tenant` для wiring.

Трекеры, не поддерживающие спринты (например, Linear), могут возвращать `ErrNotSupported` из sprint-методов. Handler получит ошибку, MCP-клиент увидит `IsError: true`.

### 3.2. Интерфейс базы знаний — `internal/knowledge/`

Tracker-agnostic RAG: write path (индексация) + read path (поиск).

#### Domain types

```go
package knowledge

import "time"

type Document struct {
    TenantID   string    // разделение данных между клиентами
    Source     string    // "jira", "youtrack", "confluence", ...
    ProjectKey string
    DocKey     string    // PROJ-123, page-id, etc.
    Title      string
    Status     string    // опционально, пусто для не-issue документов
    Author     string
    Content    string    // flat text для embedding
    Embedding  []float32
    UpdatedAt  time.Time
}

type Hit struct {
    DocKey  string
    Title   string
    Status  string
    Score   float32
    Excerpt string
}

type Filter struct {
    TenantID   string
    ProjectKey string
    Source     string // опционально: "jira", "confluence", "" = все
}
```

#### Интерфейсы

```go
type Writer interface {
    Upsert(ctx context.Context, docs []Document) error
    ReplaceProject(ctx context.Context, tenantID, projectKey string, docs []Document) error
}

type Reader interface {
    Search(ctx context.Context, queryEmbedding []float32, f Filter, topK int) ([]Hit, error)
}

type Store interface {
    Writer
    Reader
    Stats(ctx context.Context, tenantID, projectKey string) (int, error)
    Close() error
}
```

#### Связь с handlers

Текущий `handlers.KnowledgeRetriever` сохраняет сигнатуру, но возвращает `knowledge.Hit` вместо `handlers.Hit`:

```go
// internal/handlers/knowledge.go
type KnowledgeRetriever interface {
    Search(ctx context.Context, projectKey, query string, topK int) ([]knowledge.Hit, error)
}
```

Retriever (`internal/knowledge/retriever/`) оборачивает `knowledge.Store` + `embed.Embedder`, добавляя `TenantID` из context.

### 3.3. Multi-tenant — `internal/tenant/`

#### Config

```go
type Config struct {
    Name          string            // из keys.yaml: "Alice"
    TrackerType   string            // "jira", "youtrack"
    TrackerConfig map[string]string // base_url, email, token, auth_type, ...
    ProjectKeys   []string          // доступные проекты
}

type Registry struct {
    tenants map[string]*Tenant // key name → tenant
}

type Tenant struct {
    Config    Config
    Provider  tracker.Provider
    Knowledge knowledge.Store    // shared store, но TenantID подставляется автоматически
}

func (r *Registry) Resolve(keyName string) (*Tenant, error)
```

#### keys.yaml — расширенный формат

```yaml
keys:
  - key: "sk-test-alice-abc123"
    name: "Alice"
    tracker: jira
    tracker_config:
      base_url: "https://alice-corp.atlassian.net"
      email: "alice@corp.com"
      api_token: "secret-token"
      auth_type: "basic"
    projects: ["PROJ", "OPS"]

  - key: "sk-test-bob-def456"
    name: "Bob"
    tracker: jira
    tracker_config:
      base_url: "https://bob-inc.atlassian.net"
      email: "bob@inc.com"
      api_token: "another-token"
      auth_type: "basic"
    projects: ["BACKEND"]
```

Обратная совместимость: если `tracker` не указан — используется legacy формат (key + name), трекер берётся из env.

#### Wiring в register

```go
// Вместо:
//   mcp.AddTool(srv, &tool, adapt(handlers.ListIssues(jc)))
// Теперь:
//   mcp.AddTool(srv, &tool, adaptTenant(registry, func(t *tenant.Tenant) Handler[In,Out] {
//       return handlers.ListIssues(t.Provider)
//   }))
```

`adaptTenant`:
1. Достаёт `keyName` из context (положен `MultiKeyMiddleware`)
2. Резолвит `*Tenant` через `Registry`
3. Создаёт handler с провайдером тенанта
4. Вызывает handler

### 3.4. Миграция pgvector

Goose-миграция:

```sql
ALTER TABLE issue_embeddings ADD COLUMN tenant_id TEXT NOT NULL DEFAULT '';
ALTER TABLE issue_embeddings ADD COLUMN source TEXT NOT NULL DEFAULT 'jira';
ALTER TABLE issue_embeddings RENAME COLUMN issue_key TO doc_key;

-- Обновить unique constraint
ALTER TABLE issue_embeddings DROP CONSTRAINT IF EXISTS issue_embeddings_project_key_issue_key_key;
ALTER TABLE issue_embeddings ADD CONSTRAINT issue_embeddings_tenant_project_doc_key
    UNIQUE (tenant_id, project_key, doc_key);
```

Существующие данные: `tenant_id=''`, `source='jira'` — продолжают работать в single-tenant режиме.

### 3.5. Структура пакетов — итоговая

```
internal/
  tracker/
    tracker.go              types + interfaces (Issue, Provider, ...)
    jira/
      client.go             текущий internal/jira/ переехавший сюда
  knowledge/
    knowledge.go            Document, Hit, Filter, Store interfaces
    embed/                  Voyage, OpenAI, ONNX embedders (без изменений)
    index/                  Indexer: tracker.DocIterator → embed → Store
    pgvector/               реализация Store (текущий rag/store)
    retriever/              Embedder + Store → Search(query)
  tenant/
    tenant.go               Config, Tenant, Registry
    loader.go               парсинг расширенного keys.yaml
  auth/                     без изменений
  handlers/                 импорт jira.* → tracker.*, остальное без изменений
  register/                 adaptTenant, резолвинг из context
  config/                   упрощается: общие env (DATABASE_URL, embedder)
cmd/
  server/main.go            создаёт Registry → register
  index/main.go             --tenant=Alice --project=PROJ | legacy env mode
```

**Удаляемые пакеты:** `internal/jira/` (→ `tracker/jira/`), `internal/rag/` (→ `knowledge/`).

### 3.6. Обратная совместимость

| Сценарий | Поведение |
|----------|-----------|
| `MCP_API_KEY` задан, `MCP_KEYS_FILE` нет | Single-tenant, трекер из env (как сейчас) |
| `MCP_KEYS_FILE` без `tracker` в записях | Multi-key, но один трекер из env |
| `MCP_KEYS_FILE` с `tracker` в записях | Full multi-tenant |
| `--transport=stdio` | Single-tenant, трекер из env (как сейчас) |
| CLI `index --project=ABC` без `--tenant` | Трекер из env, `tenant_id=''` |
| CLI `index --tenant=Alice --project=PROJ` | Трекер из tenant config |

### 3.7. Тестирование

- **Unit-тесты handlers:** fake-реализации узких интерфейсов из `tracker` — как сейчас, но с `tracker.Issue` вместо `jira.Issue`
- **Unit-тесты tenant:** загрузка YAML, резолвинг, ошибки
- **Unit-тесты knowledge:** fake Store, проверка Filter с TenantID/Source
- **Integration-тесты pgvector:** testcontainers, проверка миграции, multi-tenant queries
- **E2E:** один сервер, два ключа с разными tracker_config, проверка изоляции данных

### 3.8. Env-матрица

| Переменная | Когда нужна |
|-----------|------------|
| `DATABASE_URL` | всегда |
| `RAG_EMBEDDER` | всегда (default `voyage`) |
| `VOYAGE_API_KEY` / `OPENAI_API_KEY` / `ONNX_MODEL_PATH` | по значению `RAG_EMBEDDER` |
| `JIRA_BASE_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN` | single-tenant mode (без `MCP_KEYS_FILE` с tracker) |
| `MCP_API_KEY` | single-key http mode |
| `MCP_KEYS_FILE` | multi-key / multi-tenant http mode |
| `MCP_ADDR` | http mode (default `:8080`) |
