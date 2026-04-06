package jira

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPClient_DoSetsBasicAuthAndAccept(t *testing.T) {
	const (
		email = "user@example.com"
		token = "secret-token"
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем метод
		require.Equal(t, http.MethodGet, r.Method)

		// Проверяем Accept header
		require.Equal(t, "application/json", r.Header.Get("Accept"))

		// Проверяем Authorization header
		authHeader := r.Header.Get("Authorization")
		require.True(t, strings.HasPrefix(authHeader, "Basic "), "Authorization должен начинаться с 'Basic '")

		encoded := strings.TrimPrefix(authHeader, "Basic ")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		require.NoError(t, err)
		require.Equal(t, email+":"+token, string(decoded))

		// Проверяем путь
		require.Contains(t, r.URL.RequestURI(), "/rest/api/3/anything")
		require.Contains(t, r.URL.RawQuery, "x=1")

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewHTTPClient(ts.URL, email, token, nil)
	resp, err := c.do(context.Background(), http.MethodGet, "/rest/api/3/anything?x=1", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func TestHTTPClient_DoTrimsBaseURLSlash(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Двойного слеша быть не должно
		require.False(t, strings.Contains(r.URL.Path, "//"), "в пути не должно быть двойного слеша: %s", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// baseURL с trailing slash
	c := NewHTTPClient(ts.URL+"/", "u", "t", nil)
	resp, err := c.do(context.Background(), http.MethodGet, "/rest/api/3/something", nil)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func TestCheckStatus(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		wantErr   bool
		errContains []string
	}{
		{
			name:    "200 OK — no error",
			status:  http.StatusOK,
			wantErr: false,
		},
		{
			name:        "400 Bad Request — error",
			status:      http.StatusBadRequest,
			wantErr:     true,
			errContains: []string{"GET", "/rest/api/3/search/jql", "400"},
		},
		{
			name:        "599 — error",
			status:      599,
			wantErr:     true,
			errContains: []string{"POST", "/rest/api/3/something", "599"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tc.status}
			var method, path string
			switch tc.status {
			case 599:
				method, path = "POST", "/rest/api/3/something"
			default:
				method, path = "GET", "/rest/api/3/search/jql"
			}
			err := checkStatus(resp, method, path)
			if tc.wantErr {
				require.Error(t, err)
				for _, want := range tc.errContains {
					require.Contains(t, err.Error(), want)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHTTPClient_DoNilHTTPClient(t *testing.T) {
	// Работаем в package jira — видны unexported поля.
	// Когда httpClient=nil, конструктор должен подставить http.DefaultClient.
	c := NewHTTPClient("https://example.atlassian.net", "u", "t", nil)
	require.NotNil(t, c.http, "c.http не должен быть nil при передаче nil в конструктор")
}
