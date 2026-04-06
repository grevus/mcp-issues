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
		// Теперь сервер обслуживает и /comment endpoints
		if strings.HasSuffix(r.URL.Path, "/comment") {
			_, _ = w.Write([]byte(`{"comments": []}`))
			return
		}
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
	searchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Обслуживаем /comment endpoints
		if strings.HasSuffix(r.URL.Path, "/comment") {
			_, _ = w.Write([]byte(`{"comments": []}`))
			return
		}
		searchCount++
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
	require.Equal(t, 2, searchCount, "должно быть ровно 2 поисковых HTTP-запроса")

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

func TestIterateIssueDocs_WithComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/3/search/jql":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
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
			}`))
		case "/rest/api/3/issue/ABC-1/comment":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"comments": [
					{"body": "First comment"},
					{"body": "Second comment"}
				]
			}`))
		case "/rest/api/3/issue/ABC-2/comment":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"comments": []}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	out, errCh := client.IterateIssueDocs(context.Background(), "ABC")

	docs, err := collectDocs(t, out, errCh)
	require.NoError(t, err)
	require.Len(t, docs, 2)

	// ABC-1: два комментария в правильном порядке
	require.Equal(t, "ABC-1", docs[0].Key)
	require.Equal(t, []string{"First comment", "Second comment"}, docs[0].Comments)

	// ABC-2: пустой массив комментариев
	require.Equal(t, "ABC-2", docs[1].Key)
	require.Empty(t, docs[1].Comments)
}

func TestExtractStatusHistory(t *testing.T) {
	t.Run("nil changelog returns nil", func(t *testing.T) {
		require.Nil(t, extractStatusHistory(nil))
	})

	t.Run("no status items returns nil", func(t *testing.T) {
		cl := &docsChangelog{
			Histories: []docsHistoryEntry{
				{
					Created: "2026-01-15T10:30:00.000+0000",
					Items: []docsHistoryItem{
						{Field: "summary", FromString: "Old", ToString: "New"},
					},
				},
			},
		}
		require.Nil(t, extractStatusHistory(cl))
	})

	t.Run("status items are extracted correctly", func(t *testing.T) {
		cl := &docsChangelog{
			Histories: []docsHistoryEntry{
				{
					Created: "2026-01-15T10:30:00.000+0000",
					Items: []docsHistoryItem{
						{Field: "status", FromString: "To Do", ToString: "In Progress"},
					},
				},
				{
					Created: "2026-01-20T12:00:00.000+0000",
					Items: []docsHistoryItem{
						{Field: "summary", FromString: "Old", ToString: "New"},
						{Field: "status", FromString: "In Progress", ToString: "Done"},
					},
				},
			},
		}
		got := extractStatusHistory(cl)
		require.Equal(t, []string{
			"2026-01-15: To Do → In Progress",
			"2026-01-20: In Progress → Done",
		}, got)
	})

	t.Run("unparseable date is skipped", func(t *testing.T) {
		cl := &docsChangelog{
			Histories: []docsHistoryEntry{
				{
					Created: "not-a-date",
					Items: []docsHistoryItem{
						{Field: "status", FromString: "To Do", ToString: "In Progress"},
					},
				},
				{
					Created: "2026-01-20T12:00:00.000+0000",
					Items: []docsHistoryItem{
						{Field: "status", FromString: "In Progress", ToString: "Done"},
					},
				},
			},
		}
		got := extractStatusHistory(cl)
		require.Equal(t, []string{"2026-01-20: In Progress → Done"}, got)
	})
}

func TestIterateIssueDocs_StatusHistory(t *testing.T) {
	const fixtureWithChangelog = `{
		"issues": [
			{
				"key": "ABC-1",
				"fields": {
					"summary": "First issue",
					"status": {"name": "Done"},
					"assignee": {"displayName": "Alice"},
					"description": "lorem ipsum",
					"updated": "2026-01-20T12:00:00.000+0000"
				},
				"changelog": {
					"histories": [
						{
							"created": "2026-01-15T10:30:00.000+0000",
							"items": [
								{"field": "status", "fromString": "To Do", "toString": "In Progress"}
							]
						},
						{
							"created": "2026-01-20T12:00:00.000+0000",
							"items": [
								{"field": "summary", "fromString": "Old", "toString": "New"},
								{"field": "status", "fromString": "In Progress", "toString": "Done"}
							]
						}
					]
				}
			}
		]
	}`

	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/comment") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"comments": []}`))
			return
		}
		capturedURL = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fixtureWithChangelog))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	out, errCh := client.IterateIssueDocs(context.Background(), "ABC")

	docs, err := collectDocs(t, out, errCh)
	require.NoError(t, err)
	require.Len(t, docs, 1)

	require.Equal(t, []string{
		"2026-01-15: To Do → In Progress",
		"2026-01-20: In Progress → Done",
	}, docs[0].StatusHistory)

	require.Contains(t, capturedURL, "expand=changelog",
		"request URL должен содержать expand=changelog")
}

func TestIterateIssueDocs_CommentsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/comment") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errorMessages":["comment fetch error"]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"issues": [
				{
					"key": "ABC-1",
					"fields": {
						"summary": "First issue",
						"status": {"name": "In Progress"},
						"assignee": null,
						"description": null,
						"updated": "2026-01-15T10:30:00.000+0000"
					}
				}
			]
		}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	out, errCh := client.IterateIssueDocs(context.Background(), "ABC")

	docs, err := collectDocs(t, out, errCh)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
	require.Empty(t, docs)
}

// TestExtractLinkedIssues — unit-тест чистой функции extractLinkedIssues.
func TestExtractLinkedIssues(t *testing.T) {
	tests := []struct {
		name  string
		links []docsIssueLink
		want  []string
	}{
		{
			name:  "nil input returns nil",
			links: nil,
			want:  nil,
		},
		{
			name:  "empty slice returns nil",
			links: []docsIssueLink{},
			want:  nil,
		},
		{
			name: "only outward issues",
			links: []docsIssueLink{
				{OutwardIssue: &docsLinkedIssue{Key: "ABC-2"}},
				{OutwardIssue: &docsLinkedIssue{Key: "ABC-4"}},
			},
			want: []string{"ABC-2", "ABC-4"},
		},
		{
			name: "only inward issues",
			links: []docsIssueLink{
				{InwardIssue: &docsLinkedIssue{Key: "ABC-3"}},
				{InwardIssue: &docsLinkedIssue{Key: "ABC-5"}},
			},
			want: []string{"ABC-3", "ABC-5"},
		},
		{
			name: "mix of outward and inward",
			links: []docsIssueLink{
				{OutwardIssue: &docsLinkedIssue{Key: "ABC-2"}},
				{InwardIssue: &docsLinkedIssue{Key: "ABC-3"}},
				{OutwardIssue: &docsLinkedIssue{Key: "ABC-4"}},
			},
			want: []string{"ABC-2", "ABC-3", "ABC-4"},
		},
		{
			name: "element with both nil pointers is skipped",
			links: []docsIssueLink{
				{OutwardIssue: &docsLinkedIssue{Key: "ABC-2"}},
				{},
				{InwardIssue: &docsLinkedIssue{Key: "ABC-3"}},
			},
			want: []string{"ABC-2", "ABC-3"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractLinkedIssues(tc.links)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestIterateIssueDocs_LinkedIssues проверяет, что issuelinks из ответа Jira
// правильно маппятся в doc.LinkedIssues.
func TestIterateIssueDocs_LinkedIssues(t *testing.T) {
	const fixtureWithLinks = `{
		"issues": [
			{
				"key": "ABC-1",
				"fields": {
					"summary": "Issue with links",
					"status": {"name": "In Progress"},
					"assignee": null,
					"description": null,
					"updated": "2026-01-15T10:30:00.000+0000",
					"issuelinks": [
						{"outwardIssue": {"key": "ABC-2"}},
						{"inwardIssue":  {"key": "ABC-3"}},
						{"outwardIssue": {"key": "ABC-4"}}
					]
				}
			}
		]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/comment") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"comments": []}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fixtureWithLinks))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	out, errCh := client.IterateIssueDocs(context.Background(), "ABC")

	docs, err := collectDocs(t, out, errCh)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, []string{"ABC-2", "ABC-3", "ABC-4"}, docs[0].LinkedIssues)
}

// TestIterateIssueDocs_CommentsErrorHasContext проверяет, что ошибка из
// fetchIssueComments содержит ключ issue для диагностики.
func TestIterateIssueDocs_CommentsErrorHasContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/comment") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errorMessages":["server error"]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"issues": [
				{
					"key": "ABC-1",
					"fields": {
						"summary": "First issue",
						"status": {"name": "In Progress"},
						"assignee": null,
						"description": null,
						"updated": "2026-01-15T10:30:00.000+0000"
					}
				}
			]
		}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	out, errCh := client.IterateIssueDocs(context.Background(), "ABC")

	_, err := collectDocs(t, out, errCh)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ABC-1", "ошибка должна содержать ключ issue")
}
