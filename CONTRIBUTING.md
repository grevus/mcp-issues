# Contributing

Thanks for considering a contribution to mcp-jira.

## Development setup

**Prerequisites:**
- Go 1.26+
- C toolchain (Xcode CLT on macOS, `build-essential` on Linux) — required for CGO (sqlite-vec, ONNX)
- Docker (only for pgvector integration tests)

**Build:**

```bash
make build          # both binaries into bin/
# or:
go build -o bin/mcp-jira ./cmd/server
go build -o bin/mcp-jira-index ./cmd/index
```

**Tests:**

```bash
make test                   # unit tests, fast
make test-integration       # + pgvector via testcontainers (needs Docker)
```

## Adding a new tool

The architecture is deliberately flat — no plugin registry, no DI container. Adding a tool is four files:

1. **Jira endpoint** — method on tracker provider + DTO in `internal/tracker/jira/`:
   ```go
   func (c *HTTPClient) GetWorklogs(ctx context.Context, issueKey string) ([]Worklog, error)
   ```

2. **Handler** — `internal/handlers/<thing>.go` with `Input`, `Output`, narrow interface, constructor:
   ```go
   type WorklogReader interface {
       GetWorklogs(ctx context.Context, issueKey string) ([]jira.Worklog, error)
   }
   func Worklogs(r WorklogReader) Handler[WorklogInput, WorklogOutput] { ... }
   ```
   **Never** import `mcp` or `echo` from handlers — only `context`, `fmt`, `encoding/json`, domain types.

3. **Test** — `<thing>_test.go` with a fake implementation of the narrow interface. Table-driven, no mocking frameworks.

4. **Register** — one line in `internal/register/register.go`:
   ```go
   mcp.AddTool(srv, &mcp.Tool{Name: "get_worklogs", Description: "..."}, adapt(handlers.Worklogs(jc)))
   ```

5. **Docs** — copy `docs/tools/_template.md` to `docs/tools/<name>.md`, fill it in, add a row to `docs/tools/README.md`. The `Description` in `register.go` and the `.md` must match.

## Code style

- `gofmt` / `goimports` on save.
- Error wrapping: `fmt.Errorf("context: %w", err)`.
- Keep interfaces narrow (one handler = one capability).
- No comments restating what the code says — only for non-obvious intent.
- No emojis.

## Invariants (do not break)

- Dependency direction: `cmd → register → handlers → narrow interfaces → providers/stores`. Handlers must never import MCP or Echo.
- `internal/register` is the only bridge to `go-sdk/mcp`.
- Echo only in `cmd/server`. `internal/auth` stays `net/http`-compatible.
- **Stdio mode: never write to stdout** (reserved for JSON-RPC). Use `log.*` (stderr) only.
- `MCP_API_KEY` required only for `--transport=http`.

## Out of scope

Don't add these without a separate issue/discussion:
- Postgres-backed API key storage (moving beyond current YAML-file multi-tenant)
- Billing / SaaS bits
- Prometheus / full observability stack
- Incremental indexing, background scheduler, Jira webhooks
- LLM generation inside handlers (all tool outputs are deterministic today)
- Confluence connector (requires its own design)

## Submitting a PR

1. Fork + branch from `master`.
2. `make test` passes.
3. `go vet ./...` clean.
4. Keep PRs focused — one feature or fix per PR.
5. Include a one-line rationale in the PR description.

## License

By contributing you agree that your contributions will be licensed under the MIT License.
