package jira

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HTTPClient — базовый HTTP-клиент для Jira REST API.
// Все методы добавляют basic auth и Accept: application/json.
type HTTPClient struct {
	baseURL string // https://you.atlassian.net (без trailing slash)
	email   string
	token   string
	http    *http.Client
}

// NewHTTPClient создаёт HTTPClient. Если httpClient == nil, используется http.DefaultClient.
// trailing slash в baseURL убирается автоматически.
func NewHTTPClient(baseURL, email, token string, httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		email:   email,
		token:   token,
		http:    httpClient,
	}
}

// do выполняет HTTP-запрос к Jira API, добавляя basic auth и Accept: application/json.
// path — относительный путь, начинающийся с "/" (например, "/rest/api/3/search/jql?jql=...").
// body может быть nil.
// Вызывающий обязан закрыть resp.Body.
func (c *HTTPClient) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("jira: build request: %w", err)
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: do request: %w", err)
	}
	return resp, nil
}

// checkStatus возвращает ошибку, если resp.StatusCode >= 400. Body НЕ закрывается.
func checkStatus(resp *http.Response, method, path string) error {
	if resp.StatusCode >= 400 {
		return fmt.Errorf("jira: %s %s -> %d", method, path, resp.StatusCode)
	}
	return nil
}
