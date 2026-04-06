package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// searchResponse — приватный DTO для парсинга ответа /rest/api/3/search/jql.
type searchResponse struct {
	Issues []issueResponse `json:"issues"`
}

type issueResponse struct {
	Key    string      `json:"key"`
	Fields issueFields `json:"fields"`
}

type issueFields struct {
	Summary  string          `json:"summary"`
	Status   issueStatus     `json:"status"`
	Assignee *issueAssignee  `json:"assignee"`
}

type issueStatus struct {
	Name string `json:"name"`
}

type issueAssignee struct {
	DisplayName string `json:"displayName"`
}

// ListIssues возвращает список задач Jira для указанного проекта.
// В этой версии всегда запрашивает maxResults=25.
func (c *HTTPClient) ListIssues(ctx context.Context, p ListIssuesParams) ([]Issue, error) {
	if err := validateProjectKey(p.ProjectKey); err != nil {
		return nil, fmt.Errorf("jira: ListIssues: %w", err)
	}

	jql := "project = " + quoteJQL(p.ProjectKey)

	q := url.Values{}
	q.Set("jql", jql)
	q.Set("fields", "summary,status,assignee")
	q.Set("maxResults", "25")

	path := "/rest/api/3/search/jql?" + q.Encode()

	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("jira: ListIssues: decode response: %w", err)
	}

	issues := make([]Issue, 0, len(sr.Issues))
	for _, ir := range sr.Issues {
		assignee := ""
		if ir.Fields.Assignee != nil {
			assignee = ir.Fields.Assignee.DisplayName
		}
		issues = append(issues, Issue{
			Key:      ir.Key,
			Summary:  ir.Fields.Summary,
			Status:   ir.Fields.Status.Name,
			Assignee: assignee,
		})
	}
	return issues, nil
}
