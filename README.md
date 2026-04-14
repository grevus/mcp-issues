# mcp-jira

> An MCP server written in Go that gives LLM clients (Claude Desktop, Cursor, Claude Web, etc.) a set of practical tools over your Jira instance — plus semantic search (RAG) over indexed issues.

[![Go Version](https://img.shields.io/badge/go-1.26%2B-00ADD8.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[Русский README →](README.ru.md)

---

## Features

Ten tools that combine live Jira calls with RAG over an indexed corpus of issues:

| Tool | Description |
|---|---|
| `list_issues` | JQL search via Jira REST API v3. |
| `get_sprint_health` | Active sprint health metrics (Jira Software / Agile API). |
| `search_jira_knowledge` | Semantic search over indexed issues. |
| `similar_issues` | Find semantically similar issues — duplicate detection / incident correlation. |
| `sprint_health_report` | Extended sprint report: risk level, blocked items, action items, scope changes. |
| `standup_digest` | Async standup: done / in-progress / blocked grouped by time window. |
| `engineering_qa` | Answer engineering questions with RAG citations. |
| `incident_context` | Incident context: similar past incidents, suspected causes, recommended checks. |
| `ticket_triage` | Suggest owning team and priority based on similar issues. |
| `release_risk_check` | Release risk assessment by `fixVersion` + postmortem search. |

Per-tool contracts: [`docs/tools/`](docs/tools/).

Transports:
- **stdio** — for Claude Desktop, Cursor, Claude Code.
- **Streamable HTTP** on `/mcp` with static API key — for Claude Web, remote clients, multi-tenant setups.

---

## Quick Start (local, no Docker)

The default storage backend is **SQLite + [sqlite-vec](https://github.com/asg017/sqlite-vec)** — no external database required. Requires a C toolchain (Xcode CLT on macOS, `build-essential` on Linux) because of CGO.

### 1. Install

```bash
go install github.com/grevus/mcp-jira/cmd/server@latest
go install github.com/grevus/mcp-jira/cmd/index@latest
```

This drops `server` and `index` binaries into `$(go env GOPATH)/bin`. Rename them if you prefer (e.g. `mcp-jira`, `mcp-jira-index`).

Or build from source:

```bash
git clone https://github.com/grevus/mcp-jira.git
cd mcp-jira
go build -o bin/mcp-jira ./cmd/server
go build -o bin/mcp-jira-index ./cmd/index
```

### 2. Configure

Copy `.env.example` to `.env` and fill in Jira + embedder credentials:

```bash
cp .env.example .env
```

Minimum required variables:

```bash
JIRA_BASE_URL=https://your-org.atlassian.net
JIRA_EMAIL=you@example.com
JIRA_API_TOKEN=your-jira-api-token

RAG_EMBEDDER=voyage
VOYAGE_API_KEY=your-voyage-api-key
```

Get a Jira API token at [id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens).
Get a Voyage AI key at [dash.voyageai.com](https://dash.voyageai.com) (free tier: 200M tokens).

### 3. Migrate + index

```bash
bin/mcp-jira-index migrate
bin/mcp-jira-index index --project=ABC
```

Database file will be created at `~/.mcp-jira/knowledge.db` (override with `SQLITE_PATH`).

### 4. Run

```bash
# stdio (Claude Desktop / Cursor)
bin/mcp-jira --transport=stdio

# HTTP (Claude Web, remote clients) — requires MCP_API_KEY
MCP_API_KEY=your-secret-key bin/mcp-jira --transport=http
```

### 5. Claude Desktop config

Add to `claude_desktop_config.json` (macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mcp-jira": {
      "command": "/absolute/path/to/bin/mcp-jira",
      "args": ["--transport=stdio"],
      "env": {
        "JIRA_BASE_URL": "https://your-org.atlassian.net",
        "JIRA_EMAIL": "you@example.com",
        "JIRA_API_TOKEN": "your-jira-api-token",
        "RAG_EMBEDDER": "voyage",
        "VOYAGE_API_KEY": "your-voyage-key"
      }
    }
  }
}
```

Restart Claude Desktop. You should see the 10 tools under the mcp-jira server.

---

## Advanced: pgvector backend (Docker)

For production or large corpora (>100k issues), use Postgres + pgvector instead of SQLite.

```bash
docker compose up -d

export KNOWLEDGE_STORE=pgvector
export DATABASE_URL=postgres://mcp:mcp@localhost:15432/mcp

bin/mcp-jira-index migrate
bin/mcp-jira-index index --project=ABC
bin/mcp-jira --transport=stdio
```

---

## Configuration

All configuration is via environment variables (or a `.env` file in the working directory).

### Jira

| Variable | Required | Default | Description |
|---|---|---|---|
| `JIRA_BASE_URL` | yes | — | e.g. `https://your-org.atlassian.net` |
| `JIRA_API_TOKEN` | yes | — | Jira API token or DC Personal Access Token |
| `JIRA_EMAIL` | yes (if `basic` auth) | — | User email for Atlassian Cloud |
| `JIRA_AUTH_TYPE` | no | `basic` | `basic` (Cloud) or `bearer` (Jira DC PAT) |

### Knowledge store

| Variable | Required | Default | Description |
|---|---|---|---|
| `KNOWLEDGE_STORE` | no | `sqlite` | `sqlite` or `pgvector` |
| `SQLITE_PATH` | no | `~/.mcp-jira/knowledge.db` | SQLite DB file path |
| `DATABASE_URL` | yes (if `pgvector`) | — | Postgres DSN, e.g. `postgres://mcp:mcp@localhost:15432/mcp` |

### Embedder

Embedding dimension is fixed at **1024**. Choose one provider:

| Variable | Required | Default | Description |
|---|---|---|---|
| `RAG_EMBEDDER` | no | `voyage` | `voyage`, `openai`, or `onnx` |
| `VOYAGE_API_KEY` | if `voyage` | — | [voyageai.com](https://voyageai.com) API key (free tier available) |
| `OPENAI_API_KEY` | if `openai` | — | OpenAI API key (uses `text-embedding-3-small` @ 1024 dims) |
| `ONNX_MODEL_PATH` | if `onnx` | — | Path to directory containing `model.onnx` (fully local, no API calls) |
| `ONNX_LIB_DIR` | no | — | Path to ONNX runtime library dir (optional) |

### Transport

| Variable | Required | Default | Description |
|---|---|---|---|
| `MCP_ADDR` | no (http only) | `:8080` | HTTP listen address |
| `MCP_API_KEY` | yes (http single-tenant) | — | API key for `/mcp` endpoint auth |
| `MCP_KEYS_FILE` | no (http multi-tenant) | — | Path to YAML with per-tenant API keys and tracker configs |

---

## Indexing

The indexer fetches all issues in a project via JQL pagination, embeds each one, and stores them in the knowledge store.

```bash
bin/mcp-jira-index index --project=ABC
```

Multi-tenant mode (keys file):

```bash
bin/mcp-jira-index index --project=ABC --tenant=acme --keys-file=./keys.yaml
```

Re-indexing is idempotent — `ReplaceProject` atomically deletes and re-inserts all documents for that project key.

No built-in scheduler. Run via cron or CI, e.g.:

```cron
0 */6 * * * /path/to/bin/mcp-jira-index index --project=ABC >> /var/log/mcp-jira-index.log 2>&1
```

---

## Architecture

```
cmd/server          stdio | streamable-http (Echo)
cmd/index           migrate | index --project=ABC
  └─ internal/register          only importer of go-sdk/mcp
       └─ internal/handlers     pure business logic, knows nothing about mcp/echo
            └─ narrow interfaces (IssueLister, SprintReader, ...)
                 ├─ internal/tracker/jira     Jira REST/Agile client
                 └─ internal/knowledge        Store interface + Retriever
                      ├─ internal/knowledge/embed     Voyage / OpenAI / ONNX
                      ├─ internal/knowledge/pgvector  Postgres + pgvector
                      ├─ internal/knowledge/sqlite    SQLite + sqlite-vec
                      └─ internal/knowledge/index     Indexer (CLI)
  └─ internal/auth              stdlib middleware, constant-time key compare
  └─ internal/config            mode-aware env validation
```

Handlers take narrow interfaces, not a fat client — each tool is trivially unit-testable with a fake.

More context in [CLAUDE.md](CLAUDE.md).

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to add a new tool, run tests, and submit PRs.

```bash
go test ./...                          # unit tests
go test -tags=integration ./...        # + pgvector via testcontainers (needs Docker)
```

---

## License

MIT — see [LICENSE](LICENSE).
