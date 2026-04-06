package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSprintHealth_HappyPath(t *testing.T) {
	var capturedPath string
	var capturedQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"values": [{"id": 42, "name": "Sprint 5", "state": "active"}]}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	result, err := client.GetSprintHealth(context.Background(), 42)
	require.NoError(t, err)

	require.Equal(t, "/rest/agile/1.0/board/42/sprint", capturedPath)
	require.Equal(t, "state=active", capturedQuery)

	require.Equal(t, 42, result.BoardID)
	require.Equal(t, "Sprint 5", result.SprintName)
	require.Equal(t, 0, result.Total)
	require.Equal(t, 0, result.Done)
	require.Equal(t, 0, result.InProgress)
	require.Equal(t, 0, result.Blocked)
	require.Equal(t, 0.0, result.Velocity)
}

func TestGetSprintHealth_NoActiveSprint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"values": []}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
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

	client := NewHTTPClient(srv.URL, "user@example.com", "token", nil)
	_, err := client.GetSprintHealth(context.Background(), 42)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
}
