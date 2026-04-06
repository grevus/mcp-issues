package register

import (
	"context"

	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/jira"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// JiraClient объединяет узкие интерфейсы всех обработчиков, работающих с Jira.
// *jira.HTTPClient автоматически удовлетворяет этому интерфейсу.
type JiraClient interface {
	ListIssues(ctx context.Context, p jira.ListIssuesParams) ([]jira.Issue, error)
	GetSprintHealth(ctx context.Context, boardID int) (jira.SprintHealth, error)
}

// Register регистрирует все MCP-инструменты в srv.
// jc — клиент Jira; ret — retriever, реализующий handlers.KnowledgeRetriever.
func Register(srv *mcp.Server, jc JiraClient, ret handlers.KnowledgeRetriever) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_issues",
		Description: "Search Jira issues using JQL filters (project, status, assignee).",
	}, adapt(handlers.ListIssues(jc)))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_sprint_health",
		Description: "Return health metrics for the active sprint of a Jira Software board.",
	}, adapt(handlers.SprintHealth(jc)))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_jira_knowledge",
		Description: "Semantic search over indexed Jira issues for a given project.",
	}, adapt(handlers.SearchKnowledge(ret)))
}
