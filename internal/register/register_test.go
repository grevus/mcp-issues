package register

import (
	"context"
	"testing"

	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/tenant"
	"github.com/grevus/mcp-jira/internal/tracker"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// fakeProvider реализует tracker.Provider с stub-ами.
type fakeProvider struct{}

func (f *fakeProvider) ListIssues(_ context.Context, _ tracker.ListParams) ([]tracker.Issue, error) {
	return []tracker.Issue{{Key: "TST-1", Summary: "Stub issue"}}, nil
}

func (f *fakeProvider) GetIssue(_ context.Context, key string) (tracker.Issue, string, error) {
	return tracker.Issue{Key: key, Summary: "Stub issue"}, "stub description", nil
}

func (f *fakeProvider) GetSprintHealth(_ context.Context, _ int) (tracker.SprintHealth, error) {
	return tracker.SprintHealth{SprintName: "Sprint 1", Total: 5, Done: 2}, nil
}

func (f *fakeProvider) GetSprintReport(_ context.Context, boardID, _ int) (tracker.SprintReport, error) {
	return tracker.SprintReport{
		Health:        tracker.SprintHealth{BoardID: boardID, SprintName: "Sprint 1", Total: 5, Done: 2},
		BlockedIssues: []tracker.Issue{},
		ScopeAdded:    []tracker.Issue{},
		ScopeRemoved:  []tracker.Issue{},
	}, nil
}

func (f *fakeProvider) GetSprintScopeChanges(_ context.Context, _ int) ([]string, []string, error) {
	return nil, nil, nil
}

func (f *fakeProvider) GetIssueComments(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (f *fakeProvider) IterateIssueDocs(_ context.Context, _ string) (<-chan tracker.IssueDoc, <-chan error) {
	ch := make(chan tracker.IssueDoc)
	errCh := make(chan error)
	close(ch)
	close(errCh)
	return ch, errCh
}

// fakeRetriever реализует tenant.KnowledgeRetriever.
type fakeRetriever struct{}

func (f *fakeRetriever) Search(_ context.Context, _, _ string, _ int) ([]knowledge.Hit, error) {
	return []knowledge.Hit{{DocKey: "TST-1", Title: "Stub", Score: 0.9}}, nil
}

// newTestServer создаёт mcp.Server и вызывает Register с fake-тенантом.
// Тенант регистрируется под ключом "" (пустая строка) — именно это значение
// возвращает auth.KeyNameFromContext в контексте без аутентификации,
// что позволяет smoke-тестам работать без HTTP middleware.
func newTestServer(t *testing.T) *mcp.Server {
	t.Helper()
	reg := tenant.NewRegistry()
	reg.Register("", &tenant.Tenant{
		Config:    tenant.Config{Name: "default"},
		Provider:  &fakeProvider{},
		Retriever: &fakeRetriever{},
	})
	srv := mcp.NewServer(impl, nil)
	Register(srv, reg)
	return srv
}

// callTool вспомогательная функция: подключает клиент к серверу через in-memory
// транспорт и вызывает инструмент по имени с переданными аргументами.
// keyName передаётся в контекст через auth.ctxKey — для multi-tenant резолвинга
// мы используем внутренний пакетный тип через adaptTenant. В тестах мы обходим
// это, регистрируя тенанта под ключом "default" и оставляя keyName пустой строкой,
// либо напрямую устанавливая имя в контексте.
func callTool(t *testing.T, srv *mcp.Server, toolName string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()

	_, err := srv.Connect(ctx, st, nil)
	require.NoError(t, err)

	client := mcp.NewClient(impl, nil)
	cs, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

// TestRegister_ListIssues — smoke-тест: инструмент list_issues зарегистрирован
// и возвращает не-ошибочный результат.
func TestRegister_ListIssues(t *testing.T) {
	srv := newTestServer(t)
	result := callTool(t, srv, "list_issues", map[string]any{
		"project_key": "TST",
	})
	require.False(t, result.IsError, "expected successful result")
	require.NotEmpty(t, result.Content)
}

// TestRegister_GetSprintHealth — smoke-тест: инструмент get_sprint_health
// зарегистрирован и возвращает не-ошибочный результат.
func TestRegister_GetSprintHealth(t *testing.T) {
	srv := newTestServer(t)
	result := callTool(t, srv, "get_sprint_health", map[string]any{
		"board_id": 42,
	})
	require.False(t, result.IsError, "expected successful result")
	require.NotEmpty(t, result.Content)
}

// TestRegister_SearchJiraKnowledge — smoke-тест: инструмент search_jira_knowledge
// зарегистрирован и возвращает не-ошибочный результат.
func TestRegister_SearchJiraKnowledge(t *testing.T) {
	srv := newTestServer(t)
	result := callTool(t, srv, "search_jira_knowledge", map[string]any{
		"project_key": "TST",
		"query":       "authentication bug",
		"top_k":       3,
	})
	require.False(t, result.IsError, "expected successful result")
	require.NotEmpty(t, result.Content)
}
