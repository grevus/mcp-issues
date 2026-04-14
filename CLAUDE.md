# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Goal

MCP server (Go) giving LLM clients a set of tools over Jira + RAG (semantic search over indexed issues).

**10 tools implemented:**
- `list_issues` — JQL search via Jira REST API v3.
- `get_sprint_health` — active sprint health metrics (Jira Software / Agile API).
- `search_jira_knowledge` — semantic search over indexed issues (RAG).
- `similar_issues` — find semantically similar issues for duplicate detection / incident correlation.
- `sprint_health_report` — extended sprint report: risk level, blocked, action items, scope changes.
- `standup_digest` — async standup: done / in-progress / blocked grouped by time window.
- `engineering_qa` — answer engineering questions with RAG citations.
- `incident_context` — incident context: similar past incidents, suspected causes, recommended checks.
- `ticket_triage` — suggest owning team and priority based on similar issues.
- `release_risk_check` — release risk assessment by fixVersion + postmortem search.

Tool contracts: `docs/tools/` (per-tool md + index). Description in `internal/register/register.go` must match the md file.

Transports: stdio (Claude Desktop/Cursor) and Streamable HTTP `/mcp` with API key (Claude Web etc.).

## Tech stack

- **Go 1.26+**, two binaries: `mcp-jira` (server) and `mcp-jira-index` (CLI indexer).
- `github.com/modelcontextprotocol/go-sdk` — MCP server. Real API: `mcp.NewServer(&Implementation{}, opts)`, `mcp.AddTool[In,Out](srv, &Tool{}, h)`, `srv.Run(ctx, &StdioTransport{})`, `mcp.NewStreamableHTTPHandler(...)`. **SSE and `srv.AddTool(name,desc,fn)` are hallucinated API — do not use.**
- `github.com/labstack/echo/v4` — HTTP only in `cmd/server`.
- **Knowledge store**: `KNOWLEDGE_STORE=sqlite` (default, local file via sqlite-vec) or `pgvector` (Postgres + pgvector).
- **Embedders**: Voyage AI (default), OpenAI, ONNX (local). Choice via `RAG_EMBEDDER=voyage|openai|onnx`. Dimension fixed at 1024.
- `github.com/pressly/goose/v3` — migrations (pgvector mode).
- `testcontainers-go` for pgvector integration tests (build tag `+integration`).
- `stretchr/testify/require` for assertions.
- Jira REST API v3 / Agile API v1.0 over `net/http`.

## Architecture

Dependencies flow top-down only. MCP SDK and Echo do not leak into business logic.

```
cmd/server/main.go            stdio | streamable-http (Echo)
cmd/index/main.go             migrate | index --project=ABC
  └─ internal/register        ONLY importer of go-sdk/mcp; adapt[In,Out] + Register
       └─ internal/handlers   Handler[In,Out] func(ctx, In) (Out, error); knows nothing about mcp/echo
            └─ narrow interfaces (IssueLister, IssueFetcher, SprintReader, SprintReporter, ...)
                 ├─ internal/tracker/jira     (HTTPClient: Jira REST/Agile)
                 └─ internal/knowledge        (Store interface + Retriever)
                      ├─ internal/knowledge/embed     (Voyage, OpenAI, ONNX)
                      ├─ internal/knowledge/pgvector  (PgvectorStore)
                      ├─ internal/knowledge/sqlite    (SqliteStore)
                      └─ internal/knowledge/index     (Indexer for CLI)
  └─ internal/auth            stdlib func(http.Handler) http.Handler, constant-time
  └─ internal/config          Load(mode) — mode-aware validation (stdio|http|index)
```

**Invariants:**
- `internal/handlers` imports only `context`/`json`/`fmt` + domain types. **Never** `mcp` or `echo`.
- `internal/register` is the only bridge to `go-sdk/mcp`. SDK updates — changes only here.
- Echo only in `cmd/server`. `auth.Middleware` stays stdlib-compatible.
- `internal/knowledge/*` knows nothing about MCP — reusable subsystem.
- Each handler takes a **narrow** interface, not a fat `jira.Client`.
- **Stdio mode: NEVER write to stdout** (reserved for JSON-RPC). Use `log.*` (stderr) only.
- `MCP_API_KEY` required **only** for `--transport=http`.

## Common commands

```bash
# Build
go build -o bin/mcp-jira ./cmd/server
go build -o bin/mcp-jira-index ./cmd/index

# SQLite mode (default, no Docker needed)
bin/mcp-jira-index migrate
bin/mcp-jira-index index --project=ABC
bin/mcp-jira --transport=stdio

# pgvector mode (requires Docker)
docker compose up -d
KNOWLEDGE_STORE=pgvector DATABASE_URL=postgres://mcp:mcp@localhost:15432/mcp bin/mcp-jira-index migrate
KNOWLEDGE_STORE=pgvector DATABASE_URL=postgres://mcp:mcp@localhost:15432/mcp bin/mcp-jira --transport=stdio

# Tests
go test ./...                                 # unit tests
go test -tags=integration ./...               # + pgvector via testcontainers (needs Docker)
```

## Adding a new tool

1. Method on tracker provider + DTO in `internal/tracker/jira/`.
2. File `internal/handlers/<thing>.go`: `Input`, `Output`, narrow interface, constructor.
3. Test with fake implementation of the narrow interface.
4. One line in `internal/register/register.go`: `mcp.AddTool(srv, &mcp.Tool{Name: "..."}, adapt(...))`.
5. `docs/tools/<name>.md` from template `docs/tools/_template.md` + line in `docs/tools/README.md`.

No plugin/registry/DI container — intentional.

## Out of scope

Do not add without a separate plan: Postgres API key storage (multi-tenant), Stripe billing, production Docker/deploy, Prometheus, incremental indexing, background scheduler, Jira webhooks, retry on 429, hybrid search, chunking, live index reload. Separately: **Confluence connector** (Phase 3), **LLM generation inside handlers** (all texts are deterministic).

## Docs

- **`docs/tools/`** — canonical MCP tool contracts (per-tool md + index). Edit together with `internal/register/register.go`.
