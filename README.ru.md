# mcp-jira

> Спроси у Claude *«что заблокировано в спринте?»* — и получи реальные данные из Jira.

[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MCP](https://img.shields.io/badge/MCP-compatible-blueviolet)](https://modelcontextprotocol.io)

Работает с **Claude Desktop · Cursor · Cline · Claude Code · VS Code Copilot** — с любым клиентом, говорящим на MCP.

MCP-сервер на Go: набор практичных инструментов поверх Jira плюс семантический поиск (RAG) по индексированному корпусу issue.

[English README →](README.md)

---

## Demo

![demo](assets/demo.gif)

<!-- TODO: записать GIF на 15–25 секунд:
  1. Пользователь пишет в Claude: "Что заблокировано в спринте ABC?"
  2. Claude вызывает get_sprint_health
  3. В ответ — реальные данные спринта
-->

---

## Tools

| Tool | Что делает | Пример промпта |
|---|---|---|
| `list_issues` | JQL-поиск по проекту / статусу / assignee / labels | *«Покажи все открытые баги на Alice в проекте ABC»* |
| `get_sprint_health` | Метрики активного спринта: done / in-progress / blocked / velocity | *«Как идёт текущий спринт на доске 42?»* |
| `search_jira_knowledge` | Семантический поиск по индексированным issue (RAG) | *«Найди issue, похожие на таймаут аутентификации»* |
| `similar_issues` | Поиск дубликатов и корреляция инцидентов | *«Есть что-то похожее на ABC-1234?»* |
| `sprint_health_report` | Расширенный отчёт: риск, блокеры, action items, scope changes | *«Дай полный отчёт по рискам текущего спринта»* |
| `standup_digest` | Асинхронный standup по окну времени | *«Что команда сделала за последние 24 часа?»* |
| `engineering_qa` | Ответы на технические вопросы с RAG-цитатами | *«Как мы чинили rate-limit баг в payments?»* |
| `incident_context` | Похожие прошлые инциденты, вероятные причины, что проверить | *«В проде таймаут по БД — что проверить?»* |
| `ticket_triage` | Предложение owning team и приоритета по похожим issue | *«Какая команда должна взять этот тикет и с каким приоритетом?»* |
| `release_risk_check` | Оценка риска релиза по `fixVersion` + поиск постмортемов | *«Какие риски у релиза 2.4.0?»* |

Контракты по каждому tool — [`docs/tools/`](docs/tools/).

Транспорты:
- **stdio** — для Claude Desktop, Cursor, Claude Code.
- **Streamable HTTP** на `/mcp` со статическим API-ключом — для Claude Web, remote-клиентов, multi-tenant.

---

## Быстрый старт

**Prerequisites:** Go 1.26+, Jira API token, ключ Voyage/OpenAI (или локальная модель ONNX). Нужен C-тулчейн (Xcode CLT на macOS, `build-essential` на Linux) из-за CGO (sqlite-vec).

```bash
# 1. Установка (или сборка из исходников)
go install github.com/grevus/mcp-jira/cmd/server@latest
go install github.com/grevus/mcp-jira/cmd/index@latest

# 2. Конфигурация — скопировать и заполнить credentials для Jira и embedder
cp .env.example .env

# 3. Миграции + индексация проекта
mcp-jira-index migrate
mcp-jira-index index --project=ABC

# 4. Запуск (stdio для десктопных клиентов)
mcp-jira --transport=stdio
```

Минимальный `.env`:

```bash
JIRA_BASE_URL=https://your-org.atlassian.net
JIRA_EMAIL=you@example.com
JIRA_API_TOKEN=your-jira-api-token

RAG_EMBEDDER=voyage
VOYAGE_API_KEY=your-voyage-api-key
```

Jira API token: [id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens).
Voyage AI: [dash.voyageai.com](https://dash.voyageai.com) (free tier — 200M токенов). **Примечание:** `api.voyageai.com` недоступен из России без VPN — используйте `openai` или `onnx` embedder.

По умолчанию хранилище — SQLite, файл БД создаётся в `~/.mcp-jira/knowledge.db` (переопределяется через `SQLITE_PATH`). Docker не нужен.

### Подключение к Claude Desktop

Добавьте в `claude_desktop_config.json` (на macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mcp-jira": {
      "command": "/absolute/path/to/mcp-jira",
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

Перезапустите Claude Desktop — под сервером `mcp-jira` появятся 10 tools.

Для HTTP-транспорта (Claude Web, remote-клиенты):

```bash
MCP_API_KEY=your-secret-key mcp-jira --transport=http
```

---

## Продвинутый режим: pgvector (Docker)

Для продакшена или больших корпусов (>100k issue) используйте Postgres + pgvector вместо SQLite.

```bash
docker compose up -d

export KNOWLEDGE_STORE=pgvector
export DATABASE_URL=postgres://mcp:mcp@localhost:15432/mcp

mcp-jira-index migrate
mcp-jira-index index --project=ABC
mcp-jira --transport=stdio
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
mcp-jira-index index --project=ABC
```

Multi-tenant режим (keys file):

```bash
mcp-jira-index index --project=ABC --tenant=acme --keys-file=./keys.yaml
```

Переиндексация идемпотентна — `ReplaceProject` атомарно удаляет и вставляет все документы проекта в одной транзакции.

Встроенного планировщика нет. Запускайте через cron или CI:

```cron
0 */6 * * * /path/to/mcp-jira-index index --project=ABC >> /var/log/mcp-jira-index.log 2>&1
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

## Как добавить новый tool

Архитектура намеренно плоская — нет plugin registry и DI-контейнера. Новый tool — это 4 файла:

1. Метод + DTO в `internal/tracker/jira/`
2. Хендлер в `internal/handlers/<thing>.go` (~30 строк, пример — `issues.go`)
3. Одна строка регистрации в `internal/register/register.go`
4. Документация: скопировать `docs/tools/_template.md` → `docs/tools/<name>.md`

Полная инструкция — в [CONTRIBUTING.md](CONTRIBUTING.md). Для старта ищите лейбл `good first issue` в GitHub Issues.

---

## Вклад

Тесты, стиль кода и PR workflow — в [CONTRIBUTING.md](CONTRIBUTING.md).

```bash
go test ./...                          # unit tests
go test -tags=integration ./...        # + pgvector через testcontainers (нужен Docker)
```

---

## Лицензия

MIT — см. [LICENSE](LICENSE).
