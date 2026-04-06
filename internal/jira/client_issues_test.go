package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const fixtureSearchResponse = `{
  "issues": [
    {
      "key": "ABC-1",
      "fields": {
        "summary": "First issue",
        "status": {"name": "In Progress"},
        "assignee": {"displayName": "Alice"}
      }
    },
    {
      "key": "ABC-2",
      "fields": {
        "summary": "Second issue",
        "status": {"name": "Done"},
        "assignee": null
      }
    }
  ]
}`

func TestListIssues_HappyPath(t *testing.T) {
	var capturedPath string
	var capturedQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fixtureSearchResponse))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	issues, err := client.ListIssues(context.Background(), ListIssuesParams{ProjectKey: "ABC"})
	require.NoError(t, err)

	// Проверяем path
	require.Equal(t, "/rest/api/3/search/jql", capturedPath)

	// Парсим query для проверок
	require.Contains(t, capturedQuery, "jql=")
	require.Contains(t, capturedQuery, "fields=")
	require.Contains(t, capturedQuery, "maxResults=")

	// Декодируем query вручную
	queryMap := parseQuery(t, capturedQuery)
	require.Contains(t, queryMap["jql"], `project = "ABC"`)
	require.Equal(t, "summary,status,assignee", queryMap["fields"])
	require.Equal(t, "25", queryMap["maxResults"])

	// Проверяем результат
	require.Len(t, issues, 2)

	require.Equal(t, "ABC-1", issues[0].Key)
	require.Equal(t, "First issue", issues[0].Summary)
	require.Equal(t, "In Progress", issues[0].Status)
	require.Equal(t, "Alice", issues[0].Assignee)

	require.Equal(t, "ABC-2", issues[1].Key)
	require.Equal(t, "Second issue", issues[1].Summary)
	require.Equal(t, "Done", issues[1].Status)
	require.Equal(t, "", issues[1].Assignee)
}

func TestListIssues_InvalidProjectKey(t *testing.T) {
	client := NewHTTPClient("http://example.com", "user@example.com", "token", nil)
	_, err := client.ListIssues(context.Background(), ListIssuesParams{ProjectKey: "abc"})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "invalid project key"), "expected 'invalid project key' in error: %s", err)
}

func TestListIssues_Filters(t *testing.T) {
	cases := []struct {
		name           string
		params         ListIssuesParams
		wantJQLContains []string
		wantMaxResults  string
	}{
		{
			name:            "only status",
			params:          ListIssuesParams{ProjectKey: "ABC", Status: "Done"},
			wantJQLContains: []string{`project = "ABC"`, `status = "Done"`},
			wantMaxResults:  "25",
		},
		{
			name:            "only assignee",
			params:          ListIssuesParams{ProjectKey: "ABC", Assignee: "Alice"},
			wantJQLContains: []string{`project = "ABC"`, `assignee = "Alice"`},
			wantMaxResults:  "25",
		},
		{
			name:            "both status and assignee with limit=50",
			params:          ListIssuesParams{ProjectKey: "ABC", Status: "In Progress", Assignee: "Bob", Limit: 50},
			wantJQLContains: []string{`project = "ABC"`, `status = "In Progress"`, `assignee = "Bob"`},
			wantMaxResults:  "50",
		},
		{
			name:            "limit=0 defaults to 25",
			params:          ListIssuesParams{ProjectKey: "ABC", Limit: 0},
			wantJQLContains: []string{`project = "ABC"`},
			wantMaxResults:  "25",
		},
		{
			name:            "limit=200 clamped to 100",
			params:          ListIssuesParams{ProjectKey: "ABC", Limit: 200},
			wantJQLContains: []string{`project = "ABC"`},
			wantMaxResults:  "100",
		},
		{
			name:            "limit=-5 defaults to 25",
			params:          ListIssuesParams{ProjectKey: "ABC", Limit: -5},
			wantJQLContains: []string{`project = "ABC"`},
			wantMaxResults:  "25",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"issues":[]}`))
			}))
			defer srv.Close()

			client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
			_, err := client.ListIssues(context.Background(), tc.params)
			require.NoError(t, err)

			queryMap := parseQuery(t, capturedQuery)

			for _, want := range tc.wantJQLContains {
				require.Contains(t, queryMap["jql"], want, "jql должен содержать %q", want)
			}
			require.Equal(t, tc.wantMaxResults, queryMap["maxResults"])
		})
	}
}

func TestListIssues_Error401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorMessages":["Unauthorized"]}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	issues, err := client.ListIssues(context.Background(), ListIssuesParams{ProjectKey: "ABC"})
	require.Error(t, err)
	require.Nil(t, issues)
	require.Contains(t, err.Error(), "GET")
	require.Contains(t, err.Error(), "/rest/api/3/search/jql")
	require.Contains(t, err.Error(), "401")
}

func TestListIssues_Error500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errorMessages":["Internal Server Error"]}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	issues, err := client.ListIssues(context.Background(), ListIssuesParams{ProjectKey: "ABC"})
	require.Error(t, err)
	require.Nil(t, issues)
	require.Contains(t, err.Error(), "GET")
	require.Contains(t, err.Error(), "/rest/api/3/search/jql")
	require.Contains(t, err.Error(), "500")
}

// parseQuery разбирает raw query string в map с URL-декодированием значений.
func parseQuery(t *testing.T, raw string) map[string]string {
	t.Helper()
	vals, err := url.ParseQuery(raw)
	require.NoError(t, err)
	result := make(map[string]string, len(vals))
	for k, v := range vals {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}
