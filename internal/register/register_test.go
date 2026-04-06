package register

import (
	"context"
	"testing"

	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/jira"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// fakeJira реализует JiraClient с помощью stub-ов.
type fakeJira struct{}

func (f *fakeJira) ListIssues(_ context.Context, _ jira.ListIssuesParams) ([]jira.Issue, error) {
	return []jira.Issue{{Key: "TST-1", Summary: "Stub issue"}}, nil
}

func (f *fakeJira) GetSprintHealth(_ context.Context, _ int) (jira.SprintHealth, error) {
	return jira.SprintHealth{SprintName: "Sprint 1", Total: 5, Done: 2}, nil
}

// fakeRetriever реализует handlers.KnowledgeRetriever.
type fakeRetriever struct{}

func (f *fakeRetriever) Search(_ context.Context, _, _ string, _ int) ([]handlers.Hit, error) {
	return []handlers.Hit{{IssueKey: "TST-1", Summary: "Stub", Score: 0.9}}, nil
}

// newTestServer создаёт mcp.Server и вызывает Register с fake-зависимостями.
func newTestServer(t *testing.T) *mcp.Server {
	t.Helper()
	srv := mcp.NewServer(impl, nil)
	Register(srv, &fakeJira{}, &fakeRetriever{})
	return srv
}

// callTool вспомогательная функция: подключает клиент к серверу через in-memory
// транспорт и вызывает инструмент по имени с переданными аргументами.
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
