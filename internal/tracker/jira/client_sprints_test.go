package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSprintHealth_HappyPath(t *testing.T) {
	var capturedSprintPath string
	var capturedSprintQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.HasPrefix(r.URL.Path, "/rest/agile/1.0/board/") {
			capturedSprintPath = r.URL.Path
			capturedSprintQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"values": [{"id": 42, "name": "Sprint 5", "state": "active"}]}`))
		} else {
			// /rest/agile/1.0/sprint/42/issue — пустой список
			_, _ = w.Write([]byte(`{"issues": []}`))
		}
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", "basic", nil)
	result, err := client.GetSprintHealth(context.Background(), 42)
	require.NoError(t, err)

	require.Equal(t, "/rest/agile/1.0/board/42/sprint", capturedSprintPath)
	require.Equal(t, "state=active", capturedSprintQuery)

	require.Equal(t, 42, result.BoardID)
	require.Equal(t, "Sprint 5", result.SprintName)
	require.Equal(t, 0, result.Total)
	require.Equal(t, 0, result.Done)
	require.Equal(t, 0, result.InProgress)
	require.Equal(t, 0, result.Blocked)
	require.Equal(t, 0.0, result.Velocity)
}

func TestGetSprintHealth_Aggregation(t *testing.T) {
	// Фикстура: 5 задач — 2 done, 1 in progress, 1 blocked (indeterminate), 1 to do
	const issueFixture = `{
		"issues": [
			{
				"key": "ABC-1",
				"fields": {
					"status": {"name": "Done", "statusCategory": {"key": "done"}},
					"customfield_10016": 3
				}
			},
			{
				"key": "ABC-2",
				"fields": {
					"status": {"name": "Done", "statusCategory": {"key": "done"}},
					"customfield_10016": 5
				}
			},
			{
				"key": "ABC-3",
				"fields": {
					"status": {"name": "In Progress", "statusCategory": {"key": "indeterminate"}},
					"customfield_10016": null
				}
			},
			{
				"key": "ABC-4",
				"fields": {
					"status": {"name": "Blocked", "statusCategory": {"key": "indeterminate"}},
					"customfield_10016": null
				}
			},
			{
				"key": "ABC-5",
				"fields": {
					"status": {"name": "To Do", "statusCategory": {"key": "new"}},
					"customfield_10016": null
				}
			}
		]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch {
		case strings.HasPrefix(r.URL.Path, "/rest/agile/1.0/board/"):
			_, _ = w.Write([]byte(`{"values": [{"id": 7, "name": "Sprint 3", "state": "active"}]}`))
		case strings.HasPrefix(r.URL.Path, "/rest/agile/1.0/sprint/7/issue"):
			_, _ = w.Write([]byte(issueFixture))
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", "basic", nil)
	result, err := client.GetSprintHealth(context.Background(), 42)
	require.NoError(t, err)

	require.Equal(t, 42, result.BoardID)
	require.Equal(t, "Sprint 3", result.SprintName)
	require.Equal(t, 5, result.Total)
	require.Equal(t, 2, result.Done)
	// ABC-4 (Blocked) не попадает в InProgress, хотя statusCategory = indeterminate
	require.Equal(t, 1, result.InProgress)
	require.Equal(t, 1, result.Blocked)
	require.InDelta(t, 8.0, result.Velocity, 0.001)
}

func TestGetSprintHealth_NoActiveSprint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"values": []}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", "basic", nil)
	_, err := client.GetSprintHealth(context.Background(), 42)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no active sprint")
	require.Contains(t, err.Error(), "42")
}

func TestGetSprintHealth_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", "basic", nil)
	_, err := client.GetSprintHealth(context.Background(), 42)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
}
