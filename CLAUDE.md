# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Status

MVP реализован по плану `docs/superpowers/plans/2026-04-07-core-mvp-small-tasks.md`, см. ветку `feat/core-mvp`. Архитектура зафиксирована в `docs/superpowers/specs/2026-04-06-mcp-jira-design.md`. **Перед любыми изменениями читай spec — он source of truth.**

## Goal

Нишевый MCP-сервер на Go, дающий LLM-клиентам три tools поверх Jira:
- `list_issues` — JQL-поиск через `/rest/api/3/search/jql`.
- `get_sprint_health` — агрегат активного спринта (Jira Software / Agile API).
- `search_jira_knowledge` — семантический поиск по индексированным issue (RAG).

Транспорты: stdio (Claude Desktop/Cursor) и Streamable HTTP `/mcp` под API-ключом (Claude Web и т.п.).

## Tech stack

- **Go 1.26+**, два single-binary: `mcp-jira` (сервер) и `mcp-jira-index` (CLI индексатор).
- `github.com/modelcontextprotocol/go-sdk` — MCP server. Реальный API: `mcp.NewServer(&Implementation{}, opts)`, `mcp.AddTool[In,Out](srv, &Tool{}, h)`, `srv.Run(ctx, &StdioTransport{})`, `mcp.NewStreamableHTTPHandler(...)`. **SSE и `srv.AddTool(name,desc,fn)` — выдуманный API, не использовать.**
- `github.com/labstack/echo/v4` — HTTP только в `cmd/server`. `mcp.NewStreamableHTTPHandler` монтируется через `echo.WrapHandler`, `auth.Middleware` — через `echo.WrapMiddleware`.
- `github.com/jackc/pgx/v5` + **Postgres + pgvector** — RAG storage (pure Go, без CGO).
- `github.com/pressly/goose/v3` — миграции через `embed.FS`.
- **Voyage AI** (default) и **OpenAI** — embedders. Выбор через `RAG_EMBEDDER=voyage|openai`. Размерность фиксирована `vector(1024)`.
- `testcontainers-go` для интеграционных тестов pgvector (build tag `+integration`).
- `stretchr/testify/require` для ассертов.
- Jira REST API v3 / Agile API v1.0 поверх `net/http` (basic auth).

## Architecture

Зависимости только сверху вниз. MCP SDK и Echo не протекают в бизнес-логику.

```
cmd/server/main.go            stdio | streamable-http (Echo)
cmd/index/main.go             migrate | index --project=ABC
  └─ internal/register        ЕДИНСТВЕННЫЙ импортёр go-sdk/mcp; adapt[In,Out] + Register
       └─ internal/handlers   Handler[In,Out] func(ctx, In) (Out, error); НЕ знает про mcp/echo
            └─ узкие интерфейсы (IssueLister, SprintReader, KnowledgeRetriever)
                 ├─ internal/jira (HTTPClient: ListIssues, GetSprintHealth, IterateIssueDocs)
                 └─ internal/rag/retriever (Embedder + Store)
                      ├─ internal/rag/embed (Voyage, OpenAI)
                      ├─ internal/rag/store (PgvectorStore + миграции)
                      └─ internal/rag/index (Indexer для CLI)
  └─ internal/auth            stdlib func(http.Handler) http.Handler, constant-time
  └─ internal/config          Load(mode) — mode-зависимая валидация (stdio|http|index)
```

**Незыблемые правила:**
- `internal/handlers` импортирует только `context`/`json`/`fmt` + domain types. **Никогда** `mcp` или `echo`.
- `internal/register` — единственный мост к `go-sdk/mcp`. При апдейте SDK правки только здесь.
- Echo — только в `cmd/server`. `auth.Middleware` остаётся stdlib-совместимым.
- `internal/rag/*` ничего не знает про MCP — переиспользуемая подсистема.
- Каждый handler принимает **узкий** интерфейс, не толстый `jira.Client`.
- **Stdio-режим: НИКОГДА не писать в stdout** (он занят JSON-RPC). Только `log.*` (stderr). Echo не создаётся в stdio-режиме.
- `MCP_API_KEY` обязателен **только** для `--transport=http`.

## Common commands

```bash
docker compose up -d                          # pgvector локально
go build -o bin/mcp-jira ./cmd/server
go build -o bin/mcp-jira-index ./cmd/index
bin/mcp-jira-index migrate                    # goose Up
bin/mcp-jira-index index --project=ABC        # полная переиндексация (TRUNCATE+INSERT в одной транзакции)
bin/mcp-jira --transport=stdio                # для Claude Desktop / Cursor
bin/mcp-jira --transport=http                 # для Claude Web; требует MCP_API_KEY

go test ./...                                 # юниты
go test -tags=integration ./...               # + pgvector через testcontainers (нужен Docker)
```

**Env (по mode):** `JIRA_BASE_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`, `DATABASE_URL`, `VOYAGE_API_KEY` или `OPENAI_API_KEY`, `RAG_EMBEDDER` (default `voyage`), `MCP_API_KEY` (только http), `MCP_ADDR` (default `:8080`). Точная матрица — в spec §3.7.

## Adding a new Jira tool

1. Метод на `*jira.HTTPClient` + DTO в `internal/jira/`.
2. Файл `internal/handlers/<thing>.go`: `Input`, `Output`, узкий интерфейс, функция-конструктор `Handler[In,Out]`.
3. Тест с fake-реализацией узкого интерфейса.
4. Одна строка в `internal/register/register.go`: `mcp.AddTool(srv, &mcp.Tool{Name: "..."}, adapt(handlers.Foo(jc)))`.

Никакого plugin/registry/DI container — намеренный отказ.

## Out of scope

Не добавляй без отдельного плана: Postgres-хранилище API-ключей (multi-tenant), Stripe billing, продакшн Docker/деплой, Prometheus, инкрементальная индексация, фоновый scheduler в сервере, Jira webhooks, retry на 429, hybrid search, чанкинг, live reload индекса, third embedder.

## Docs

- **`docs/superpowers/specs/2026-04-06-mcp-jira-design.md`** — source of truth по архитектуре. Читай это перед любыми правками.
- `docs/superpowers/plans/2026-04-06-core-mvp.md` — исходный план реализации (историческая версия).
- `docs/superpowers/plans/2026-04-07-core-mvp-small-tasks.md` — актуальный план мелких таск (использовался при MVP-реализации).
