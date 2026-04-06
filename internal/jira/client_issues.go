package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
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

// resolveLimit нормализует значение Limit: <= 0 → 25, > 100 → 100.
func resolveLimit(limit int) int {
	switch {
	case limit <= 0:
		return 25
	case limit > 100:
		return 100
	default:
		return limit
	}
}

// ListIssues возвращает список задач Jira для указанного проекта.
// Поддерживает опциональные фильтры Status, Assignee и Limit.
func (c *HTTPClient) ListIssues(ctx context.Context, p ListIssuesParams) ([]Issue, error) {
	if err := validateProjectKey(p.ProjectKey); err != nil {
		return nil, fmt.Errorf("jira: ListIssues: %w", err)
	}

	jql := "project = " + quoteJQL(p.ProjectKey)
	if p.Status != "" {
		jql += ` AND status = ` + quoteJQL(p.Status)
	}
	if p.Assignee != "" {
		jql += ` AND assignee = ` + quoteJQL(p.Assignee)
	}

	q := url.Values{}
	q.Set("jql", jql)
	q.Set("fields", "summary,status,assignee")
	q.Set("maxResults", strconv.Itoa(resolveLimit(p.Limit)))

	path := "/rest/api/3/search/jql?" + q.Encode()

	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkStatus(resp, "GET", path); err != nil {
		return nil, err
	}

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
