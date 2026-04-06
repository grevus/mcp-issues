package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// commentsResponse — приватный DTO для парсинга ответа GET /rest/api/3/issue/{key}/comment.
type commentsResponse struct {
	Comments []commentEntry `json:"comments"`
}

type commentEntry struct {
	Body json.RawMessage `json:"body"` // может быть строкой или ADF-объектом
}

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

// parseTextOrADF извлекает текст из JSON-поля, которое может быть:
// - null (json: null) — возвращаем ""
// - строка (json: "...") — возвращаем как есть
// - ADF-объект (json: {...}) — возвращаем "" (ADF не парсится)
// Используется для description и comment.body.
func parseTextOrADF(raw json.RawMessage) string {
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

// parseDescription — псевдоним parseTextOrADF для обратной совместимости внутри пакета.
func parseDescription(raw json.RawMessage) string {
	return parseTextOrADF(raw)
}

// fetchIssueComments выполняет GET /rest/api/3/issue/{key}/comment и возвращает
// плоский список текстов комментариев. Если комментариев нет — nil.
// При ошибке HTTP или декодирования возвращает ошибку.
func (c *HTTPClient) fetchIssueComments(ctx context.Context, issueKey string) ([]string, error) {
	path := "/rest/api/3/issue/" + issueKey + "/comment"
	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp, "GET", path); err != nil {
		resp.Body.Close()
		return nil, err
	}

	var cr commentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("jira: fetchIssueComments: decode response: %w", err)
	}
	resp.Body.Close()

	if len(cr.Comments) == 0 {
		return nil, nil
	}
	texts := make([]string, 0, len(cr.Comments))
	for _, c := range cr.Comments {
		texts = append(texts, parseTextOrADF(c.Body))
	}
	return texts, nil
}

// IterateIssueDocs возвращает два канала: out с IssueDoc и errCh с ошибкой.
// Горутина проходит постранично через /rest/api/3/search/jql, отправляя каждый
// issue как IssueDoc. При успехе оба канала закрываются. При ошибке — сначала
// отправляется ошибка в errCh, затем оба канала закрываются.
// Поля StatusHistory, LinkedIssues оставляются пустыми (Tasks 18-19).
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

				comments, err := c.fetchIssueComments(ctx, ir.Key)
				if err != nil {
					errCh <- err
					return
				}

				doc := IssueDoc{
					ProjectKey:  projectKey,
					Key:         ir.Key,
					Summary:     ir.Fields.Summary,
					Status:      ir.Fields.Status.Name,
					Assignee:    assignee,
					Description: parseDescription(ir.Fields.Description),
					Comments:    comments,
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
