package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// docsSearchResponse — приватный DTO для парсинга ответа /rest/api/3/search/jql
// при итерации IssueDoc. Отличается от searchResponse наличием nextPageToken и
// расширенным набором полей.
type docsSearchResponse struct {
	Issues        []docsIssueResponse `json:"issues"`
	NextPageToken string              `json:"nextPageToken"`
}

type docsIssueResponse struct {
	Key    string         `json:"key"`
	Fields docsIssueFields `json:"fields"`
}

type docsIssueFields struct {
	Summary     string          `json:"summary"`
	Status      issueStatus     `json:"status"`
	Assignee    *issueAssignee  `json:"assignee"`
	Description json.RawMessage `json:"description"` // может быть string, null или ADF-объект
	Updated     string          `json:"updated"`
}

const updatedTimeLayout = "2006-01-02T15:04:05.000-0700"

// parseUpdated разбирает строку updated из Jira. При ошибке возвращает zero time.
func parseUpdated(s string) time.Time {
	t, err := time.Parse(updatedTimeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// parseDescription извлекает текст из поля description, которое может быть:
// - null (json: null)
// - строка (json: "...") — возвращаем как есть
// - ADF-объект (json: {...}) — возвращаем пустую строку (Task 17+)
func parseDescription(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// null
	if string(raw) == "null" {
		return ""
	}
	// строка начинается с '"'
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return ""
		}
		return s
	}
	// ADF-объект или что-то другое — пустая строка
	return ""
}

// IterateIssueDocs возвращает два канала: out с IssueDoc и errCh с ошибкой.
// Горутина проходит постранично через /rest/api/3/search/jql, отправляя каждый
// issue как IssueDoc. При успехе оба канала закрываются. При ошибке — сначала
// отправляется ошибка в errCh, затем оба канала закрываются.
// Поля Comments, StatusHistory, LinkedIssues оставляются пустыми (Tasks 17-19).
func (c *HTTPClient) IterateIssueDocs(ctx context.Context, projectKey string) (<-chan IssueDoc, <-chan error) {
	out := make(chan IssueDoc)
	errCh := make(chan error, 1)

	if err := validateProjectKey(projectKey); err != nil {
		errCh <- fmt.Errorf("jira: IterateIssueDocs: %w", err)
		close(errCh)
		close(out)
		return out, errCh
	}

	go func() {
		defer close(out)
		defer close(errCh)

		nextPageToken := ""
		for {
			// Проверяем контекст перед каждым запросом
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			q := url.Values{}
			q.Set("jql", `project="`+projectKey+`"`)
			q.Set("fields", "summary,status,assignee,description,issuelinks,updated")
			q.Set("maxResults", "100")
			if nextPageToken != "" {
				q.Set("nextPageToken", nextPageToken)
			}

			path := "/rest/api/3/search/jql?" + q.Encode()

			resp, err := c.do(ctx, "GET", path, nil)
			if err != nil {
				errCh <- err
				return
			}

			if err := checkStatus(resp, "GET", path); err != nil {
				resp.Body.Close()
				errCh <- err
				return
			}

			var sr docsSearchResponse
			if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
				resp.Body.Close()
				errCh <- fmt.Errorf("jira: IterateIssueDocs: decode response: %w", err)
				return
			}
			resp.Body.Close()

			for _, ir := range sr.Issues {
				assignee := ""
				if ir.Fields.Assignee != nil {
					assignee = ir.Fields.Assignee.DisplayName
				}
				doc := IssueDoc{
					ProjectKey:  projectKey,
					Key:         ir.Key,
					Summary:     ir.Fields.Summary,
					Status:      ir.Fields.Status.Name,
					Assignee:    assignee,
					Description: parseDescription(ir.Fields.Description),
					UpdatedAt:   parseUpdated(ir.Fields.Updated),
				}

				select {
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				case out <- doc:
				}
			}

			// Если nextPageToken пустой — это последняя страница
			if sr.NextPageToken == "" {
				return
			}
			nextPageToken = sr.NextPageToken
		}
	}()

	return out, errCh
}
