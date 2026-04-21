# mcp-issues

> Ask Claude *"what's blocked in this sprint?"* and get real Jira data back.

[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MCP](https://img.shields.io/badge/MCP-compatible-blueviolet)](https://modelcontextprotocol.io)

Works with **Claude Desktop · Cursor · Cline · Claude Code · VS Code Copilot** — anything that speaks MCP.

A Go MCP server that exposes practical tools over your Jira instance plus semantic search (RAG) over an indexed corpus of issues.

[Русский README →](README.ru.md)

---

## Demo

![demo](assets/demo.gif)

<!-- TODO: record a 15–25s GIF:
  1. User types "What's blocked in ABC sprint?" in Claude
  2. Claude calls get_sprint_health
  3. Response with real sprint data
-->

---

## Tools

| Tool | What it does | Example prompt |
|---|---|---|
| `list_issues` | Filter issues via JQL (project, status, assignee, labels) | *"Show me all open bugs assigned to Alice in ABC"* |
| `sprint_health_report` | Extended report: risk level, blockers, action items, scope changes | *"Give me a full risk report for the current sprint"* |
| `standup_digest` | Async standup grouped by time window | *"What did my team ship in the last 24h?"* |
| `engineering_qa` | Engineering Q&A with RAG citations | *"How did we handle the rate-limit bug in payments?"* |
| `get_sprint_health` | Active sprint stats: done / in-progress / blocked / velocity | *"How's the current sprint going for board 42?"* |
| `incident_context` | Similar past incidents, suspected causes, checks | *"We have a DB timeout in prod — what should I check?"* |
| `release_risk_check` | Release risk by `fixVersion` + postmortem search | *"Any risks for release 2.4.0?"* |
| `search_jira_knowledge` | Semantic search over indexed issues (RAG) | *"Find issues similar to authentication timeout"* |
| `similar_issues` | Duplicate detection and incident correlation | *"Anything that looks like ABC-1234?"* |
| `ticket_triage` | Suggest owning team and priority from similar issues | *"Which team should own this ticket and what priority?"* |

Per-tool contracts: [`docs/tools/`](docs/tools/).

Transports:
- **stdio** — for Claude Desktop, Cursor, Claude Code.
- **Streamable HTTP** on `/mcp` with a static API key — for Claude Web, remote clients, multi-tenant setups.

---

## Quickstart

**Prerequisites:** Go 1.26+, a Jira API token, a Voyage/OpenAI key (or a local ONNX model). A C toolchain (Xcode CLT on macOS, `build-essential` on Linux) is required for CGO (sqlite-vec).

```bash
# 1. Install (or build from source)
go install github.com/grevus/mcp-issues/cmd/server@latest
go install github.com/grevus/mcp-issues/cmd/index@latest

# 2. Configure — copy and fill in Jira + embedder credentials
cp .env.example .env

# 3. Migrate + index a project
mcp-issues-index migrate
mcp-issues-index index --project=ABC

# 4. Run (stdio for desktop clients)
mcp-issues --transport=stdio
```

> No Docker needed — SQLite is the default store (`~/.mcp-issues/knowledge.db`).

Minimum `.env`:

```bash
JIRA_BASE_URL=https://your-org.atlassian.net
JIRA_EMAIL=you@example.com
JIRA_API_TOKEN=your-jira-api-token

RAG_EMBEDDER=voyage
VOYAGE_API_KEY=your-voyage-api-key
```

Get a Jira token at [id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens).
Voyage AI key at [dash.voyageai.com](https://dash.voyageai.com) (free tier: 200M tokens).

SQLite is the default store — the DB lands at `~/.mcp-issues/knowledge.db` (override with `SQLITE_PATH`). No Docker needed.

### Connect to Claude Desktop

Add to `claude_desktop_config.json` (macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mcp-issues": {
      "command": "/absolute/path/to/mcp-issues",
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

Restart Claude Desktop — the 10 tools appear under the `mcp-issues` server.

For HTTP transport (Claude Web, remote clients):

```bash
MCP_API_KEY=your-secret-key mcp-issues --transport=http
```

---

## Advanced: pgvector backend (Docker)

For production or large corpora (>100k issues), use Postgres + pgvector instead of SQLite.

```bash
docker compose up -d

export KNOWLEDGE_STORE=pgvector
export DATABASE_URL=postgres://mcp:mcp@localhost:15432/mcp

mcp-issues-index migrate
mcp-issues-index index --project=ABC
mcp-issues --transport=stdio
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
| `SQLITE_PATH` | no | `~/.mcp-issues/knowledge.db` | SQLite DB file path |
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
mcp-issues-index index --project=ABC
```

Multi-tenant mode (keys file):

```bash
mcp-issues-index index --project=ABC --tenant=acme --keys-file=./keys.yaml
```

Re-indexing is idempotent — `ReplaceProject` atomically deletes and re-inserts all documents for that project key.

No built-in scheduler. Run via cron or CI, e.g.:

```cron
0 */6 * * * /path/to/mcp-issues-index index --project=ABC >> /var/log/mcp-issues-index.log 2>&1
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

## Adding a new tool

The architecture is deliberately flat — no plugin registry, no DI container. Adding a tool is 4 files:

1. Method + DTO in `internal/tracker/jira/`
2. Handler in `internal/handlers/<thing>.go` (~30 lines, see `issues.go` as a reference)
3. Register one line in `internal/register/register.go`
4. Docs: copy `docs/tools/_template.md` → `docs/tools/<name>.md`

Full details in [CONTRIBUTING.md](CONTRIBUTING.md). Good first issues: look for the `good first issue` label on GitHub Issues.

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for tests, code style, and PR workflow.

```bash
go test ./...                          # unit tests
go test -tags=integration ./...        # + pgvector via testcontainers (needs Docker)
```

---

## License

MIT — see [LICENSE](LICENSE).
