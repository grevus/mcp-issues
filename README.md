# mcp-jira

Нишевый MCP-сервер на Go, дающий LLM-клиентам три инструмента поверх Jira:
`list_issues` (JQL-поиск), `get_sprint_health` (агрегат активного спринта) и
`search_jira_knowledge` (семантический поиск по индексированным issue через RAG + pgvector).

## Quickstart

```bash
# 1. Поднять Postgres с pgvector
docker compose up -d

# 2. Собрать бинари
go build -o bin/mcp-jira ./cmd/server
go build -o bin/mcp-jira-index ./cmd/index

# 3. Применить миграции
bin/mcp-jira-index migrate

# 4. Проиндексировать проект
bin/mcp-jira-index index --project=ABC

# 5a. Запуск для Claude Desktop / Cursor (stdio)
bin/mcp-jira --transport=stdio

# 5b. Запуск для Claude Web и т.п. (Streamable HTTP)
export MCP_API_KEY=secret
bin/mcp-jira --transport=http
```

## Env matrix

| Variable | Required for | Default | Description |
|---|---|---|---|
| `JIRA_BASE_URL` | stdio, http, index | — | Базовый URL Jira, например `https://you.atlassian.net` |
| `JIRA_EMAIL` | stdio, http, index | — | Email учётной записи Jira |
| `JIRA_API_TOKEN` | stdio, http, index | — | API-токен Jira (Basic Auth) |
| `DATABASE_URL` | stdio, http, index | — | DSN Postgres, например `postgres://mcp:mcp@localhost:5432/mcp?sslmode=disable` |
| `RAG_EMBEDDER` | — | `voyage` | Выбор embedder: `voyage` или `openai` |
| `VOYAGE_API_KEY` | stdio, http, index (если `RAG_EMBEDDER=voyage`) | — | API-ключ Voyage AI |
| `OPENAI_API_KEY` | stdio, http, index (если `RAG_EMBEDDER=openai`) | — | API-ключ OpenAI |
| `MCP_API_KEY` | **только http** | — | Статический ключ для авторизации HTTP-запросов к `/mcp` |
| `MCP_ADDR` | http | `:8080` | Адрес и порт HTTP-сервера |

## Claude Desktop config

Добавьте в `claude_desktop_config.json` (обычно `~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mcp-jira": {
      "command": "/absolute/path/to/bin/mcp-jira",
      "args": ["--transport=stdio"],
      "env": {
        "JIRA_BASE_URL": "https://you.atlassian.net",
        "JIRA_EMAIL": "you@example.com",
        "JIRA_API_TOKEN": "your-api-token",
        "DATABASE_URL": "postgres://mcp:mcp@localhost:5432/mcp?sslmode=disable",
        "VOYAGE_API_KEY": "your-voyage-key"
      }
    }
  }
}
```
