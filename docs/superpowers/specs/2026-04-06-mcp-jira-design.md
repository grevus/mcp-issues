# mcp-jira — Design Spec

**Дата:** 2026-04-06
**Статус:** Approved (через brainstorming)
**Scope:** Core MVP (single iteration plan следующим шагом)

## 1. Цель и контекст

`mcp-jira` — нишевый MCP-сервер на Go, дающий LLM-клиентам (Claude Desktop, Cursor, Cline, Claude Web) умный доступ к Jira. В Core MVP — три tools: классический REST-доступ к задачам и активному спринту, плюс семантический поиск по Jira-знаниям через RAG.

Целевая аудитория: backend/platform-инженеры, поднимающие сервер для своей команды или продающие как managed-инструмент позже.

**Зачем нужен дизайн-документ:** план реализации, родившийся из roadmap-блога, опирался на выдуманный API SDK, ломал изоляцию пакетов и не учитывал реалии актуального `modelcontextprotocol/go-sdk`. Этот документ фиксирует исправленные архитектурные решения до того, как мы переписываем план.

## 2. Scope

### В scope MVP

- **Tools (3):**
  - `list_issues(project_key, status?, assigned_to?, limit?)` — JQL-поиск через `/rest/api/3/search/jql`.
  - `get_sprint_health(board_id)` — агрегация активного спринта (Jira Software / Agile API).
  - `search_jira_knowledge(project_key, query, top_k?)` — семантический поиск по индексированным issue.
- **Транспорты:** stdio (локальный) и Streamable HTTP (`/mcp`, удалённый).
- **RAG:** Postgres + pgvector, embeddings от Voyage AI (default) или OpenAI (fallback). Полная (не инкрементальная) индексация через CLI. Контент документа — `summary + description + все комментарии + история статусов + linked issues`.
- **Auth для HTTP:** статический API-ключ из env, constant-time сравнение.
- **Два бинаря:** `mcp-jira` (сервер) и `mcp-jira-index` (CLI индексатор + миграции).

### Out of scope (отдельные планы)

- Postgres-хранилище API-ключей (multi-tenant), Stripe metered billing.
- Dockerfile продакшн-образа, деплой на Railway/Fly.io, HTTPS termination.
- Prometheus/observability, structured logging.
- Инкрементальная RAG-индексация, дельта-обновление, Jira webhooks.
- Фоновый scheduler в сервере.
- Retry/backoff на 429 от Jira/Voyage/OpenAI.
- Третий embedder (Cohere/Ollama/Jina).
- Hybrid search (semantic + keyword RRF).
- Чанкинг длинных issue (пока считаем `один issue = один документ`).
- Live reload индекса в работающем сервере (после индексации — рестарт).
- Листинги на маркетплейсах, landing page.

## 3. Архитектурные решения и обоснования

### 3.1. Сигнатура handlers — Вариант 3 (typed Out + adapter)

```go
type Handler[In, Out any] func(ctx context.Context, in In) (Out, error)
```

- `internal/handlers` НЕ импортирует `modelcontextprotocol/go-sdk/mcp`. Зависимости: только `context`, `encoding/json`, `fmt`, domain-types из `internal/jira` и `internal/rag/retriever`.
- `internal/register` — единственный импортёр `mcp` — содержит generic-функцию `adapt[In, Out](h Handler[In, Out]) ToolHandlerFor[In, Out]`, которая:
  - на success: маршалит `Out` в JSON и кладёт в `*mcp.CallToolResult.Content` как `TextContent`, во второй возврат отдаёт сам `Out` для structured tool result;
  - на error: возвращает `*mcp.CallToolResult{IsError: true, Content: TextContent{err.Error()}}`, нулевое значение `Out`, `nil` в позиции `error` (MCP спецификация различает «tool вернул ошибку» и «вызов сломался»).

**Почему не Вариант 1 (handlers с честной MCP-сигнатурой):** ради изоляции — если SDK сломает API в очередной раз, правки только в `register/register.go`. Структурированные ответы handlers сохраняем за счёт типизированного `Out`.

**Почему не Вариант 2 (adapter с `string`):** теряли structured output, который нужен MCP-клиентам для programmatic-доступа. Один лишний type-параметр в `Handler` — небольшая цена.

### 3.2. Узкие интерфейсы вместо толстого `jira.Client`

Каждый handler принимает свой минимальный интерфейс:

```go
// internal/handlers/issues.go
type IssueLister interface {
    ListIssues(ctx context.Context, p jira.ListIssuesParams) ([]jira.Issue, error)
}

// internal/handlers/sprints.go
type SprintReader interface {
    GetSprintHealth(ctx context.Context, boardID int) (jira.SprintHealth, error)
}

// internal/handlers/knowledge.go
type KnowledgeRetriever interface {
    Search(ctx context.Context, q retriever.Query) ([]retriever.Hit, error)
}
```

`*jira.HTTPClient` автоматически удовлетворяет `IssueLister` и `SprintReader` (duck-typing). `*retriever.Retriever` — `KnowledgeRetriever`.

**Зачем:** упрощает unit-тесты (фейк-реализация — три строки), документирует контракт каждого handler-а явно, делает добавление новых tools механическим.

### 3.3. Транспорт для удалённого доступа — только Streamable HTTP

- `mcp.NewStreamableHTTPHandler(getServer, opts)` монтируется в Echo на `/mcp`, метод `ANY`.
- SSE как legacy транспорт сознательно НЕ поддерживаем. Если конкретный клиент окажется SSE-only — добавим `mcp.NewSSEHandler` как +5 строк (отдельный план).

### 3.4. RAG storage — Postgres + pgvector

**Решение:** `internal/rag/store/postgres.go` поверх `jackc/pgx/v5` + миграции через `pressly/goose` с `embed.FS`.

**Почему не chromem-go (был в первой версии плана):** chromem — файл с gob-сериализацией, без concurrent r/w, без durability на crash, без SQL-инспекции. Не «база» в привычном смысле.

**Почему не sqlite-vec:** требует CGO, ломает кросс-компиляцию single-binary.

**Почему не Qdrant/Weaviate:** другая mental model (не SQL), внешний сервис без преимуществ Postgres для нашего кейса.

**Цена решения:** появилась внешняя зависимость — пользователь обязан поднять Postgres. Локально — `docker run pgvector/pgvector:pg17`. В production — любой managed Postgres с расширением `vector`. Это считаем приемлемым: целевая аудитория — backend-инженеры, для них Postgres — норма.

### 3.5. Embeddings — Voyage (default) + OpenAI (fallback)

- `internal/rag/embed/embedder.go` — интерфейс.
- `internal/rag/embed/voyage.go` — модель `voyage-3` (1024-dim, $0.06/М tokens), HTTP к `api.voyageai.com`.
- `internal/rag/embed/openai.go` — модель `text-embedding-3-small` с `dimensions=1024` для совместимости со схемой Postgres.
- Выбор через env `RAG_EMBEDDER=voyage|openai`, default `voyage`.
- Размерность вектора в схеме Postgres зафиксирована на `vector(1024)` — обе модели её отдают (OpenAI — через truncation параметр).

### 3.6. Два отдельных бинаря

- `cmd/server/main.go` → `mcp-jira` (флаги: `--transport=stdio|http`).
- `cmd/index/main.go` → `mcp-jira-index` (флаги/subcommands: `migrate`, `index --project=ABC`).

**Почему не один бинарь с subcommand:** проще каждый `main.go` по отдельности, нет необходимости тащить `cobra`/`urfave/cli`. Цена — две команды в README; для нашей аудитории это ок.

### 3.7. Конфигурация — режимо-зависимая валидация

`config.Load(mode)` принимает один из режимов: `stdio`, `http`, `index`. Валидация по mode:

| env | stdio | http | index |
|---|---|---|---|
| `JIRA_BASE_URL` | required | required | required |
| `JIRA_EMAIL` | required | required | required |
| `JIRA_API_TOKEN` | required | required | required |
| `DATABASE_URL` | required | required | required |
| `RAG_EMBEDDER` | optional (default `voyage`) | optional (default `voyage`) | optional (default `voyage`) |
| `VOYAGE_API_KEY` | required если embedder=voyage | required если embedder=voyage | required если embedder=voyage |
| `OPENAI_API_KEY` | required если embedder=openai | required если embedder=openai | required если embedder=openai |
| `MCP_API_KEY` | **ignored** | **required** | ignored |
| `MCP_ADDR` | ignored | optional (default `:8080`) | ignored |

stdio-режим не требует `MCP_API_KEY` — это исправление прежней ошибки, где stdio-пользователю приходилось задавать ключ ради ничего.

### 3.8. Stdio-режим: запрет на запись в stdout

Stdio MCP использует stdout как канал JSON-RPC. Любой `fmt.Println`/`os.Stdout.Write` ломает протокол. В `cmd/server/main.go` — явный комментарий-предупреждение и правило: только `log` (он пишет в stderr). Никакого `e.Logger` в stdio-режиме (Echo вообще не создаётся).

## 4. Структура пакетов

```
cmd/
  server/main.go              # mcp-jira: stdio | streamable-http
  index/main.go               # mcp-jira-index: migrate | index
internal/
  config/
    config.go                 # Load(mode), валидация по режиму
    config_test.go
  jira/
    types.go                  # Issue, SprintHealth, IssueDoc
    client.go                 # *HTTPClient: ListIssues, GetSprintHealth, IterateIssueDocs
    client_test.go            # против httptest.Server
    jql.go                    # quoteJQL(s) с тестами на эскейпинг
    jql_test.go
  handlers/
    handler.go                # type Handler[In, Out any] func(ctx, In) (Out, error)
    issues.go                 # IssueLister + ListIssuesInput/Output + ListIssues
    issues_test.go
    sprints.go                # SprintReader + SprintHealthInput/Output + SprintHealth
    sprints_test.go
    knowledge.go              # KnowledgeRetriever + SearchKnowledgeInput/Output + SearchKnowledge
    knowledge_test.go
  register/
    register.go               # adapt[In,Out] + Register(srv, jc, ret)
    register_test.go          # adapt success/error пути
  auth/
    apikey.go                 # stdlib middleware, constant-time
    apikey_test.go
  rag/
    embed/
      embedder.go             # interface Embedder { Embed(ctx, []string) ([][]float32, error); Dimension() int }
      voyage.go               # VoyageEmbedder
      voyage_test.go          # против httptest.Server
      openai.go               # OpenAIEmbedder
      openai_test.go
    store/
      store.go                # interface Store { Upsert([]Document); Query(vec, topK, filter) []Hit; Stats }
      postgres.go             # PgvectorStore (pgx + миграции через goose)
      postgres_test.go        # testcontainers-go: pgvector контейнер
      migrations/
        001_init.sql          # CREATE EXTENSION vector + CREATE TABLE issues_index + HNSW индексы
    index/
      indexer.go              # Indexer{Reader IssueDocReader, Embedder, Store}; полная переиндексация
      indexer_test.go         # fake reader/embedder/store
    retriever/
      retriever.go            # Retriever{Embedder, Store}; Search(ctx, Query) []Hit
      retriever_test.go
README.md
CLAUDE.md
docker-compose.yml            # pgvector для локальной разработки
docs/
  superpowers/
    specs/2026-04-06-mcp-jira-design.md
    plans/2026-04-06-core-mvp.md
```

**Принципы:**

- **Один файл — одна ответственность.** Когда `internal/jira/client.go` перевалит за ~400 строк, в плане триггер на split по namespace (`issues.go`, `sprints.go`, `boards.go`).
- **Зависимости только сверху вниз.** `cmd/*` → `internal/register` → `internal/handlers` → `internal/jira` + `internal/rag/retriever` → `internal/rag/{store,embed}`. Никаких циклов, никаких импортов снизу вверх.
- **`internal/handlers` ничего не знает про MCP SDK и Echo.**
- **`internal/register` — единственный импортёр `mcp`.**
- **`internal/rag/*` ничего не знает про MCP вообще** — самостоятельная подсистема, переиспользуема.
- **Echo живёт ТОЛЬКО в `cmd/server`.**

## 5. Ключевые контракты

### 5.1. Jira HTTP клиент

```go
type ListIssuesParams struct {
    ProjectKey string
    Status     string
    Assignee   string
    Limit      int    // default 25, max 100
}

type Issue struct {
    Key      string `json:"key"`
    Summary  string `json:"summary"`
    Status   string `json:"status"`
    Assignee string `json:"assignee,omitempty"`
}

type SprintHealth struct {
    BoardID    int     `json:"board_id"`
    SprintName string  `json:"sprint_name"`
    Total      int     `json:"total"`
    Done       int     `json:"done"`
    InProgress int     `json:"in_progress"`
    Blocked    int     `json:"blocked"`
    Velocity   float64 `json:"velocity"`
}

type IssueDoc struct {
    ProjectKey    string
    Key           string
    Summary       string
    Status        string
    Assignee      string
    Description   string
    Comments      []string  // плоский список текстов
    StatusHistory []string  // "2026-01-01: To Do → In Progress"
    LinkedIssues  []string  // ключи связанных
    UpdatedAt     time.Time
}

type HTTPClient struct { /* baseURL, basic auth, *http.Client */ }

func (c *HTTPClient) ListIssues(ctx, ListIssuesParams) ([]Issue, error)
func (c *HTTPClient) GetSprintHealth(ctx, boardID int) (SprintHealth, error)
func (c *HTTPClient) IterateIssueDocs(ctx, projectKey string) (<-chan IssueDoc, <-chan error)
```

**Endpoint contract:**

- `ListIssues` → `GET /rest/api/3/search/jql?jql=...&fields=summary,status,assignee&maxResults=N` (новый, не deprecated).
- `GetSprintHealth` → `GET /rest/agile/1.0/board/{id}/sprint?state=active` + `GET /rest/agile/1.0/sprint/{sprintId}/issue?fields=status`.
- `IterateIssueDocs` → пагинированный обход `GET /rest/api/3/search/jql?jql=project=KEY&fields=summary,status,assignee,description,issuelinks,updated&expand=changelog&maxResults=100&nextPageToken=...` + per-issue `GET /rest/api/3/issue/{key}/comment`.

**JQL escaping:** `jql.go` содержит `quoteJQL(s string) string`, экранирующий `\` → `\\` и `"` → `\"`, оборачивающий результат в двойные кавычки. Юнит-тесты на: пустую строку, строку с кавычкой, строку с бэкслэшем, обычный ключ проекта.

### 5.2. RAG: Embedder

```go
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int                  // 1024 для обеих текущих реализаций
    Name() string                    // "voyage" / "openai" — для логов
}
```

Реализации делают batching внутри (Voyage: 128 текстов на запрос; OpenAI: 100). Контракт: длина выхода равна длине входа, порядок сохранён.

### 5.3. RAG: Store

```go
type Document struct {
    ProjectKey string
    IssueKey   string
    Summary    string
    Status     string
    Assignee   string
    Content    string           // плоский текст для индексации (как рендерим — в Indexer)
    Embedding  []float32
}

type Filter struct {
    ProjectKey string
}

type Hit struct {
    IssueKey string
    Summary  string
    Status   string
    Score    float32  // cosine similarity, 0..1
    Excerpt  string   // первые ~300 символов Content
}

type Store interface {
    Upsert(ctx context.Context, docs []Document) error
    Query(ctx context.Context, vec []float32, topK int, f Filter) ([]Hit, error)
    Stats(ctx context.Context, projectKey string) (count int, err error)
}
```

`PgvectorStore`:

- Конструктор `NewPgvectorStore(ctx, dsn)` — создаёт `pgxpool.Pool`.
- Никаких автоматических миграций при `New`. Миграции — только через `mcp-jira-index migrate`.
- Размерность вектора в схеме фиксирована (`vector(1024)`); если конструктор получает Embedder с другой `Dimension()` — `cmd/index` падает с понятной ошибкой ещё на старте.

### 5.4. RAG: Indexer и Retriever

```go
type IssueDocReader interface {
    IterateIssueDocs(ctx context.Context, projectKey string) (<-chan jira.IssueDoc, <-chan error)
}

type Indexer struct {
    Reader   IssueDocReader
    Embedder embed.Embedder
    Store    store.Store
}

func (i *Indexer) Reindex(ctx context.Context, projectKey string) (Stats, error)
// Полная переиндексация: TRUNCATE по project_key → загрузка всех docs → batch embed → batch upsert → COMMIT.
// В одной транзакции, чтобы при крахе индексатора в БД не осталось полу-стёртой коллекции.
```

```go
type Query struct {
    ProjectKey string
    Text       string
    TopK       int   // default 5, max 20
}

type Retriever struct {
    Embedder embed.Embedder
    Store    store.Store
}

func (r *Retriever) Search(ctx context.Context, q Query) ([]store.Hit, error)
```

### 5.5. Document rendering для RAG

`Indexer` сериализует `IssueDoc` в плоский текст по фиксированному шаблону:

```
[KEY] {Key}
Summary: {Summary}
Status: {Status}
Assignee: {Assignee}

Description:
{Description}

Comments:
- {Comment 1}
- {Comment 2}
...

Status history:
- {Entry 1}
...

Linked issues: {Key1}, {Key2}, ...
```

Этот же шаблон используется в тестах Indexer-а, рендеринг не меняется без обновления тестов.

## 6. Data flow

### 6.1. MCP tool call (server)

```
LLM client → Streamable HTTP /mcp (POST) → Echo
                                         → auth.Middleware (X-API-Key)
                                         → mcp.NewStreamableHTTPHandler(srv)
                                         → registered tool handler (через adapt)
                                         → handlers.X(narrowInterface, input)
                                         → jira.HTTPClient.* / retriever.Search
                                         → возврат typed Out
                                         → adapt маршалит в *mcp.CallToolResult{TextContent: JSON}
                                         → MCP протокол доставляет клиенту
```

stdio: тот же путь без Echo и auth — `srv.Run(ctx, &mcp.StdioTransport{})`.

### 6.2. RAG индексация (CLI)

```
mcp-jira-index migrate                       → goose.Up (embed.FS migrations)
mcp-jira-index index --project=ABC           → config.Load("index")
                                             → jira.NewHTTPClient
                                             → embed.NewVoyageEmbedder | NewOpenAIEmbedder
                                             → store.NewPgvectorStore
                                             → indexer.Reindex(ctx, "ABC"):
                                                 BEGIN
                                                 DELETE FROM issues_index WHERE project_key='ABC'
                                                 chan IssueDoc → buffer 64 → embed batch → Upsert batch
                                                 COMMIT
                                             → лог: indexed N docs in T seconds
```

### 6.3. RAG поиск (через MCP tool)

```
search_jira_knowledge(project_key="ABC", query="auth refactor", top_k=5)
   → handlers.SearchKnowledge(retriever, input)
   → retriever.Search:
        vec := embedder.Embed([query])
        store.Query(vec, top_k=5, Filter{ProjectKey:"ABC"})
            SELECT issue_key, summary, status,
                   1 - (embedding <=> $1) AS score,
                   substring(content, 1, 300) AS excerpt
            FROM issues_index
            WHERE project_key = $2
            ORDER BY embedding <=> $1
            LIMIT 5;
   → []Hit → typed Out → JSON → MCP TextContent
```

Pgvector использует cosine distance оператор `<=>`; индекс HNSW над `vector_cosine_ops`.

## 7. Обработка ошибок

- **Jira HTTP ≥400:** возврат `fmt.Errorf("jira: %s %s -> %d", method, path, status)`. На уровне handler-а ошибка уходит в `adapt` → `IsError: true` + текст. LLM получает читаемое объяснение.
- **Voyage/OpenAI HTTP ≥400:** аналогично, без ретраев в MVP. Ошибка → handler → IsError.
- **Postgres ошибки:** прокидываются как есть, оборачиваются `fmt.Errorf("pgvector: %w", err)` на уровне `PgvectorStore`.
- **Невалидный input в MCP tool:** SDK сам валидирует JSON по jsonschema, возвращает ошибку до handler-а. На уровне handler-а — только бизнес-валидация (например, `top_k > 20` — error).
- **Stdio + ошибка:** `log.Fatal` (stderr), процесс падает, MCP-клиент видит закрытие стрима.

## 8. Тестирование

| Слой | Стратегия | Примечания |
|---|---|---|
| `internal/config` | unit, `t.Setenv` | проверка валидации по mode |
| `internal/jira` | `httptest.Server`, table-driven | проверка request URL/JQL/headers/parsing |
| `internal/jira/jql` | unit, table-driven escape | edge cases: пустая, кавычки, бэкслэши |
| `internal/auth` | unit на `httptest.NewRecorder` | constant-time и три кейса (valid/invalid/missing) |
| `internal/handlers/*` | unit с ручными fake-узких-интерфейсов | без MCP SDK |
| `internal/register` | unit на `adapt`: success path и error path | проверка `IsError`, маршалинга |
| `internal/rag/embed/voyage` | `httptest.Server` | проверка batching, parsing, dimension |
| `internal/rag/embed/openai` | `httptest.Server` | то же |
| `internal/rag/store/postgres` | **testcontainers-go** с `pgvector/pgvector:pg17` | реальные SQL+pgvector queries; build tag `+integration` чтобы можно было пропускать без Docker |
| `internal/rag/index` | fake reader/embedder/store | проверка batching, рендеринга документа, корректности транзакции (через fake store с историей вызовов) |
| `internal/rag/retriever` | fake embedder + fake store | проверка передачи topK и filter |
| `cmd/server` / `cmd/index` | smoke build (`go build`) + `--help` | без интеграционного запуска в CI |

CI-стратегия: `go test ./...` без интеграций по умолчанию + `go test -tags=integration ./...` для полного прогона (требует Docker).

## 9. Конфигурация и onboarding

### 9.1. Локальный dev quickstart

```bash
git clone && cd mcp-jira
docker compose up -d                    # pgvector
go build -o bin/mcp-jira ./cmd/server
go build -o bin/mcp-jira-index ./cmd/index

export JIRA_BASE_URL=https://you.atlassian.net
export JIRA_EMAIL=you@example.com
export JIRA_API_TOKEN=...
export DATABASE_URL=postgres://mcp:mcp@localhost:5432/mcp?sslmode=disable
export VOYAGE_API_KEY=...
# либо: export RAG_EMBEDDER=openai OPENAI_API_KEY=...

bin/mcp-jira-index migrate
bin/mcp-jira-index index --project=ABC

bin/mcp-jira --transport=stdio          # для Claude Desktop / Cursor
# или
export MCP_API_KEY=secret
bin/mcp-jira --transport=http           # для Claude Web и т.п.
```

### 9.2. `docker-compose.yml`

Один сервис `pgvector/pgvector:pg17` с прокинутым портом 5432, volume для данных, env для пользователя/пароля/БД, согласованные с дефолтным `DATABASE_URL` в README.

## 10. Расширяемость: рецепт «добавить новый Jira tool»

Документируется в README и CLAUDE.md:

1. **Метод на `*jira.HTTPClient`** — добавить новый endpoint и DTO в `internal/jira/`.
2. **Файл в `internal/handlers/<thing>.go`** — описать `Input`, `Output`, узкий интерфейс (например, `type WorklogReader interface { GetWorklogs(...) }`), функцию `Worklogs(r WorklogReader) Handler[WorklogInput, WorklogOutput]`.
3. **Тест `<thing>_test.go`** — fake-реализация узкого интерфейса.
4. **Одна строка в `internal/register/register.go`:**
   ```go
   mcp.AddTool(srv, &mcp.Tool{Name: "get_worklogs", Description: "..."}, adapt(handlers.Worklogs(jc)))
   ```

Никакой плагин-системы, registry, dependency injection container — это намеренный отказ.

## 11. Известные ограничения и долги

- Полная переиндексация на каждый запуск `index`. Для проектов >10к issue это будет медленно и дорого. Дельта-индексация — отдельный план.
- Сервер кеширует pool до Postgres, но не подписывается на изменения индекса. После переиндексации сервер не нужно перезапускать (новые SELECT-ы видят свежие данные), но если меняется схема — нужно.
- Один `RAG_EMBEDDER` на всю инсталляцию. Нельзя в одной БД хранить векторы разных моделей одновременно.
- Без retry/backoff на Jira/Voyage/OpenAI 429. При нагрузке индексатор может упасть.
- JQL escaping — самописный, без покрытия всех edge-кейсов JQL-грамматики. Whitelist валидация project_key (только `[A-Z][A-Z0-9_]*`) дополнительно.
- Нет file lock на индексаторе. Два одновременных `index --project=ABC` приведут к гонке. Документируется как «не делать так».

## 12. Резолюция исправлений к первой версии плана

Первая версия плана (`docs/superpowers/plans/2026-04-06-core-mvp.md`) содержала ряд ошибок, вскрытых ревью:

| # | Проблема | Резолюция |
|---|---|---|
| 1 | Выдуманный API SDK (`mcp.WithName`, `srv.AddTool`, `srv.ServeStdio`, `srv.SSEHandler`) | Переписано на реальный API: `mcp.NewServer(&Implementation{}, opts)`, `mcp.AddTool[In, Out]`, `srv.Run(ctx, &StdioTransport{})`, `mcp.NewStreamableHTTPHandler` |
| 2 | SSE как основной транспорт | Заменён на Streamable HTTP |
| 3 | `internal/handlers` импортировал бы SDK при честной сигнатуре | Решено через Вариант 3 (typed Out + adapter в register) |
| 4 | `/rest/api/3/search` deprecated | Используется `/rest/api/3/search/jql` |
| 5 | Без `fields=` и без `maxResults` | Явный whitelist полей и default `maxResults=25` |
| 6 | JQL escaping через `%q` | Отдельная функция `quoteJQL` с тестами |
| 7 | `MCP_API_KEY` обязателен для stdio | Mode-зависимая валидация в `config.Load(mode)` |
| 8 | Толстый `jira.Client` интерфейс с заглушкой между тасками | Узкие интерфейсы под каждый handler; полная реализация клиента в одной таске |
| 9 | Stdio + log в stdout | Явный комментарий-предупреждение, Echo не создаётся в stdio-режиме |
| 10 | `internal/jira/types.go` упоминал несуществующий тип `Sprint` | Удалён |
| 11 | `internal/auth/apikey.go` упоминал несуществующий `Validator` | Удалён |
| 12 | go-atlassian упомянут в File Structure, но не используется | Удалён |
| 13 | Бинарь `mcp-jira` vs `server` несогласован | Явно `bin/mcp-jira` и `bin/mcp-jira-index` через `-o` |
| 14 | RAG отсутствовал | Добавлен как полноценная подсистема (Postgres+pgvector, Voyage/OpenAI, CLI индексатор) |

## 13. Следующий шаг

Перевыпустить план реализации `docs/superpowers/plans/2026-04-06-core-mvp.md` через skill `superpowers:writing-plans`, с учётом всех решений выше. Ожидаемый объём — ~16 тасок.
