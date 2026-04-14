# mcp-jira

> MCP-сервер на Go, дающий LLM-клиентам (Claude Desktop, Cursor, Claude Web и т.п.) набор практических инструментов поверх Jira — плюс семантический поиск (RAG) по индексированным issue.

[![Go Version](https://img.shields.io/badge/go-1.26%2B-00ADD8.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[English README →](README.md)

---

## Возможности

Десять tools, комбинирующих live-вызовы Jira и RAG по индексированному корпусу issue:

| Tool | Описание |
|---|---|
| `list_issues` | JQL-поиск через Jira REST API v3. |
| `get_sprint_health` | Метрики активного спринта (Jira Software / Agile API). |
| `search_jira_knowledge` | Семантический поиск по индексированным issue. |
| `similar_issues` | Поиск похожих issue — дубликаты / корреляция инцидентов. |
| `sprint_health_report` | Расширенный отчёт по спринту: риск, блокеры, action items, scope changes. |
| `standup_digest` | Асинхронный standup: done / in-progress / blocked за окно времени. |
| `engineering_qa` | Ответы на технические вопросы с RAG-цитатами. |
| `incident_context` | Контекст инцидента: похожие прошлые, вероятные причины, что проверить. |
| `ticket_triage` | Предложение owning team и приоритета по похожим issue. |
| `release_risk_check` | Оценка риска релиза по `fixVersion` + поиск постмортемов. |

Контракты по каждому tool — [`docs/tools/`](docs/tools/).

Транспорты:
- **stdio** — для Claude Desktop, Cursor, Claude Code.
- **Streamable HTTP** на `/mcp` со статическим API-ключом — для Claude Web, remote-клиентов, multi-tenant.

---

## Быстрый старт (локально, без Docker)

Хранилище по умолчанию — **SQLite + [sqlite-vec](https://github.com/asg017/sqlite-vec)**, внешняя БД не нужна. Требуется C-тулчейн (Xcode CLT на macOS, `build-essential` на Linux) из-за CGO.

### 1. Установка

```bash
go install github.com/grevus/mcp-jira/cmd/server@latest
go install github.com/grevus/mcp-jira/cmd/index@latest
```

Это положит бинари `server` и `index` в `$(go env GOPATH)/bin`. При желании переименуйте (например, в `mcp-jira`, `mcp-jira-index`).

Или сборка из исходников:

```bash
git clone https://github.com/grevus/mcp-jira.git
cd mcp-jira
go build -o bin/mcp-jira ./cmd/server
go build -o bin/mcp-jira-index ./cmd/index
```

### 2. Настройка

Скопируйте `.env.example` в `.env` и заполните credentials для Jira и embedder:

```bash
cp .env.example .env
```

Минимально необходимые переменные:

```bash
JIRA_BASE_URL=https://your-org.atlassian.net
JIRA_EMAIL=you@example.com
JIRA_API_TOKEN=your-jira-api-token

RAG_EMBEDDER=voyage
VOYAGE_API_KEY=your-voyage-api-key
```

Jira API token: [id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens).
Voyage AI API key: [dash.voyageai.com](https://dash.voyageai.com) (free tier — 200M токенов). **Примечание:** `api.voyageai.com` недоступен из России без VPN — используйте `openai` или `onnx` embedder.

### 3. Миграции + индексация

```bash
bin/mcp-jira-index migrate
bin/mcp-jira-index index --project=ABC
```

Файл БД создастся в `~/.mcp-jira/knowledge.db` (переопределяется через `SQLITE_PATH`).

### 4. Запуск

```bash
# stdio (Claude Desktop / Cursor)
bin/mcp-jira --transport=stdio

# HTTP (Claude Web, remote) — нужен MCP_API_KEY
MCP_API_KEY=your-secret-key bin/mcp-jira --transport=http
```

### 5. Конфиг Claude Desktop

Добавьте в `claude_desktop_config.json` (на macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`):

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

Перезапустите Claude Desktop. Под сервером mcp-jira должны появиться 10 tools.

---

## Продвинутый режим: pgvector (Docker)

Для продакшена или больших корпусов (>100k issue) используйте Postgres + pgvector вместо SQLite.

```bash
docker compose up -d

export KNOWLEDGE_STORE=pgvector
export DATABASE_URL=postgres://mcp:mcp@localhost:15432/mcp

bin/mcp-jira-index migrate
bin/mcp-jira-index index --project=ABC
bin/mcp-jira --transport=stdio
```

---

## Конфигурация

Вся конфигурация через переменные окружения (или файл `.env` в рабочей директории).

### Jira

| Переменная | Обязательна | Default | Описание |
|---|---|---|---|
| `JIRA_BASE_URL` | да | — | напр. `https://your-org.atlassian.net` |
| `JIRA_API_TOKEN` | да | — | Jira API token или DC Personal Access Token |
| `JIRA_EMAIL` | да (при `basic`) | — | Email пользователя для Atlassian Cloud |
| `JIRA_AUTH_TYPE` | нет | `basic` | `basic` (Cloud) или `bearer` (Jira DC PAT) |

### Knowledge store

| Переменная | Обязательна | Default | Описание |
|---|---|---|---|
| `KNOWLEDGE_STORE` | нет | `sqlite` | `sqlite` или `pgvector` |
| `SQLITE_PATH` | нет | `~/.mcp-jira/knowledge.db` | Путь к файлу SQLite |
| `DATABASE_URL` | да (при `pgvector`) | — | Postgres DSN, напр. `postgres://mcp:mcp@localhost:15432/mcp` |

### Embedder

Размерность эмбеддинга зафиксирована на **1024**. Выберите одного провайдера:

| Переменная | Обязательна | Default | Описание |
|---|---|---|---|
| `RAG_EMBEDDER` | нет | `voyage` | `voyage`, `openai` или `onnx` |
| `VOYAGE_API_KEY` | при `voyage` | — | [voyageai.com](https://voyageai.com) API key (есть free tier) |
| `OPENAI_API_KEY` | при `openai` | — | OpenAI API key (использует `text-embedding-3-small` @ 1024 dims) |
| `ONNX_MODEL_PATH` | при `onnx` | — | Путь к директории с `model.onnx` (полностью локальный, без API) |
| `ONNX_LIB_DIR` | нет | — | Путь к директории библиотеки ONNX runtime (опц.) |

### Transport

| Переменная | Обязательна | Default | Описание |
|---|---|---|---|
| `MCP_ADDR` | нет (только http) | `:8080` | HTTP listen address |
| `MCP_API_KEY` | да (http single-tenant) | — | API-ключ для авторизации `/mcp` |
| `MCP_KEYS_FILE` | нет (http multi-tenant) | — | Путь к YAML с per-tenant ключами и tracker-конфигами |

---

## Индексация

Индексатор забирает все issue проекта через JQL pagination, строит эмбеддинг для каждого и сохраняет в knowledge store.

```bash
bin/mcp-jira-index index --project=ABC
```

Multi-tenant режим (keys file):

```bash
bin/mcp-jira-index index --project=ABC --tenant=acme --keys-file=./keys.yaml
```

Переиндексация идемпотентна — `ReplaceProject` атомарно удаляет и вставляет все документы проекта в одной транзакции.

Встроенного планировщика нет. Запускайте через cron или CI:

```cron
0 */6 * * * /path/to/bin/mcp-jira-index index --project=ABC >> /var/log/mcp-jira-index.log 2>&1
```

---

## Архитектура

```
cmd/server          stdio | streamable-http (Echo)
cmd/index           migrate | index --project=ABC
  └─ internal/register          единственный импортёр go-sdk/mcp
       └─ internal/handlers     чистая бизнес-логика, не знает о mcp/echo
            └─ узкие интерфейсы (IssueLister, SprintReader, ...)
                 ├─ internal/tracker/jira     Jira REST/Agile client
                 └─ internal/knowledge        Store interface + Retriever
                      ├─ internal/knowledge/embed     Voyage / OpenAI / ONNX
                      ├─ internal/knowledge/pgvector  Postgres + pgvector
                      ├─ internal/knowledge/sqlite    SQLite + sqlite-vec
                      └─ internal/knowledge/index     Indexer (CLI)
  └─ internal/auth              stdlib middleware, constant-time compare
  └─ internal/config            mode-aware валидация env
```

Хендлеры принимают узкие интерфейсы, а не толстый клиент — каждый tool тривиально юнит-тестируется через fake.

Подробности — в [CLAUDE.md](CLAUDE.md).

---

## Вклад

Инструкции по добавлению tool, запуску тестов и PR — в [CONTRIBUTING.md](CONTRIBUTING.md).

```bash
go test ./...                          # unit tests
go test -tags=integration ./...        # + pgvector через testcontainers (нужен Docker)
```

---

## Лицензия

MIT — см. [LICENSE](LICENSE).
