package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const fixtureDocsPage1 = `{
  "issues": [
    {
      "key": "ABC-1",
      "fields": {
        "summary": "First issue",
        "status": {"name": "In Progress"},
        "assignee": {"displayName": "Alice"},
        "description": "lorem ipsum",
        "updated": "2026-01-15T10:30:00.000+0000"
      }
    },
    {
      "key": "ABC-2",
      "fields": {
        "summary": "Second issue",
        "status": {"name": "Done"},
        "assignee": null,
        "description": null,
        "updated": "2026-01-16T08:00:00.000+0000"
      }
    }
  ]
}`

const fixtureDocsPage1WithToken = `{
  "issues": [
    {
      "key": "ABC-1",
      "fields": {
        "summary": "First issue",
        "status": {"name": "In Progress"},
        "assignee": {"displayName": "Alice"},
        "description": "lorem ipsum",
        "updated": "2026-01-15T10:30:00.000+0000"
      }
    },
    {
      "key": "ABC-2",
      "fields": {
        "summary": "Second issue",
        "status": {"name": "Done"},
        "assignee": null,
        "description": null,
        "updated": "2026-01-16T08:00:00.000+0000"
      }
    }
  ],
  "nextPageToken": "page2"
}`

const fixtureDocsPage2 = `{
  "issues": [
    {
      "key": "ABC-3",
      "fields": {
        "summary": "Third issue",
        "status": {"name": "To Do"},
        "assignee": {"displayName": "Bob"},
        "description": "third description",
        "updated": "2026-01-17T12:00:00.000+0000"
      }
    },
    {
      "key": "ABC-4",
      "fields": {
        "summary": "Fourth issue",
        "status": {"name": "In Progress"},
        "assignee": null,
        "description": null,
        "updated": "2026-01-18T09:00:00.000+0000"
      }
    }
  ]
}`

func collectDocs(t *testing.T, out <-chan IssueDoc, errCh <-chan error) ([]IssueDoc, error) {
	t.Helper()
	var docs []IssueDoc
	for doc := range out {
		docs = append(docs, doc)
	}
	var lastErr error
	for err := range errCh {
		lastErr = err
	}
	return docs, lastErr
}

func TestIterateIssueDocs_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fixtureDocsPage1))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	out, errCh := client.IterateIssueDocs(context.Background(), "ABC")

	docs, err := collectDocs(t, out, errCh)
	require.NoError(t, err)
	require.Len(t, docs, 2)

	// Первый doc
	require.Equal(t, "ABC", docs[0].ProjectKey)
	require.Equal(t, "ABC-1", docs[0].Key)
	require.Equal(t, "First issue", docs[0].Summary)
	require.Equal(t, "In Progress", docs[0].Status)
	require.Equal(t, "Alice", docs[0].Assignee)
	require.Equal(t, "lorem ipsum", docs[0].Description)
	require.False(t, docs[0].UpdatedAt.IsZero(), "UpdatedAt должен быть заполнен")

	// Второй doc — assignee nil, description null
	require.Equal(t, "ABC-2", docs[1].Key)
	require.Equal(t, "", docs[1].Assignee)
	require.Equal(t, "", docs[1].Description)
	require.False(t, docs[1].UpdatedAt.IsZero(), "UpdatedAt должен быть заполнен")
}

func TestIterateIssueDocs_TwoPages(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("nextPageToken") == "page2" {
			_, _ = w.Write([]byte(fixtureDocsPage2))
		} else {
			_, _ = w.Write([]byte(fixtureDocsPage1WithToken))
		}
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	out, errCh := client.IterateIssueDocs(context.Background(), "ABC")

	docs, err := collectDocs(t, out, errCh)
	require.NoError(t, err)
	require.Len(t, docs, 4)
	require.Equal(t, 2, requestCount, "должно быть ровно 2 HTTP-запроса")

	keys := make([]string, 0, len(docs))
	for _, d := range docs {
		keys = append(keys, d.Key)
	}
	require.Equal(t, []string{"ABC-1", "ABC-2", "ABC-3", "ABC-4"}, keys)
}

func TestIterateIssueDocs_InvalidProjectKey(t *testing.T) {
	client := NewHTTPClient("http://example.com", "user@example.com", "token", nil)
	out, errCh := client.IterateIssueDocs(context.Background(), "abc") // lowercase — invalid

	docs, err := collectDocs(t, out, errCh)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "invalid project key"),
		"expected 'invalid project key' in error: %s", err)
	require.Empty(t, docs)
}

func TestIterateIssueDocs_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errorMessages":["Internal Server Error"]}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	out, errCh := client.IterateIssueDocs(context.Background(), "ABC")

	docs, err := collectDocs(t, out, errCh)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
	require.Empty(t, docs)
}
