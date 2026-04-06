package handlers

import (
	"context"

	"github.com/grevus/mcp-jira/internal/jira"
)

// IssueLister — узкий интерфейс, который handler ListIssues получает на вход.
// *jira.HTTPClient автоматически удовлетворяет его.
type IssueLister interface {
	ListIssues(ctx context.Context, p jira.ListIssuesParams) ([]jira.Issue, error)
}

// ListIssuesInput — параметры MCP tool list_issues.
type ListIssuesInput struct {
	ProjectKey string `json:"project_key"`
	Status     string `json:"status,omitempty"`
	AssignedTo string `json:"assigned_to,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// ListIssuesOutput — результат MCP tool list_issues.
type ListIssuesOutput struct {
	Issues []jira.Issue `json:"issues"`
}

// ListIssues возвращает Handler, оборачивающий IssueLister в типизированную функцию.
func ListIssues(l IssueLister) Handler[ListIssuesInput, ListIssuesOutput] {
	return func(ctx context.Context, in ListIssuesInput) (ListIssuesOutput, error) {
		issues, err := l.ListIssues(ctx, jira.ListIssuesParams{
			ProjectKey: in.ProjectKey,
			Status:     in.Status,
			Assignee:   in.AssignedTo,
			Limit:      in.Limit,
		})
		if err != nil {
			return ListIssuesOutput{}, err
		}
		return ListIssuesOutput{Issues: issues}, nil
	}
}
