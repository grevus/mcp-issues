package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/grevus/mcp-jira/internal/tracker"
)

var issueKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-\d+$`)

// validateIssueKey — whitelist для issue key ("ABC-123"), чтобы не прокидывать
// произвольные строки в URL.
func validateIssueKey(s string) error {
	if !issueKeyRe.MatchString(s) {
		return fmt.Errorf("jira: invalid issue key %q", s)
	}
	return nil
}

// getIssueResponse — DTO для /rest/api/3/issue/{key}.
type getIssueResponse struct {
	Key    string             `json:"key"`
	Fields getIssueRespFields `json:"fields"`
}

type getIssueRespFields struct {
	Summary     string         `json:"summary"`
	Status      issueStatus    `json:"status"`
	Assignee    *issueAssignee `json:"assignee"`
	Description string         `json:"description"` // при fields=summary,status,assignee,description приходит plain text в v3 только если expand. Оставляем best-effort.
}

// GetIssue возвращает одну задачу Jira по ключу и её описание.
// Используется в similar_issues и ticket_triage handlers.
func (c *HTTPClient) GetIssue(ctx context.Context, key string) (tracker.Issue, string, error) {
	if err := validateIssueKey(key); err != nil {
		return tracker.Issue{}, "", fmt.Errorf("jira: GetIssue: %w", err)
	}

	path := "/rest/api/3/issue/" + key + "?fields=summary,status,assignee,description"
	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return tracker.Issue{}, "", err
	}
	defer resp.Body.Close()

	if err := checkStatus(resp, "GET", path); err != nil {
		return tracker.Issue{}, "", err
	}

	var ir getIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&ir); err != nil {
		return tracker.Issue{}, "", fmt.Errorf("jira: GetIssue: decode response: %w", err)
	}

	assignee := ""
	if ir.Fields.Assignee != nil {
		assignee = ir.Fields.Assignee.DisplayName
	}
	return tracker.Issue{
		Key:      ir.Key,
		Summary:  ir.Fields.Summary,
		Status:   ir.Fields.Status.Name,
		Assignee: assignee,
	}, ir.Fields.Description, nil
}

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
func (c *HTTPClient) ListIssues(ctx context.Context, p tracker.ListParams) ([]tracker.Issue, error) {
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
	if p.FixVersion != "" {
		jql += ` AND fixVersion = ` + quoteJQL(p.FixVersion)
	}
	if p.UpdatedFrom != "" {
		if err := validateJQLDate(p.UpdatedFrom); err != nil {
			return nil, fmt.Errorf("jira: ListIssues: UpdatedFrom: %w", err)
		}
		jql += ` AND updated >= ` + quoteJQL(p.UpdatedFrom)
	}
	if p.UpdatedTo != "" {
		if err := validateJQLDate(p.UpdatedTo); err != nil {
			return nil, fmt.Errorf("jira: ListIssues: UpdatedTo: %w", err)
		}
		jql += ` AND updated <= ` + quoteJQL(p.UpdatedTo)
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

	issues := make([]tracker.Issue, 0, len(sr.Issues))
	for _, ir := range sr.Issues {
		assignee := ""
		if ir.Fields.Assignee != nil {
			assignee = ir.Fields.Assignee.DisplayName
		}
		issues = append(issues, tracker.Issue{
			Key:      ir.Key,
			Summary:  ir.Fields.Summary,
			Status:   ir.Fields.Status.Name,
			Assignee: assignee,
		})
	}
	return issues, nil
}
