package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grevus/mcp-jira/internal/auth"
	"github.com/stretchr/testify/require"
)

func TestMiddleware_ValidKey(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.Middleware("secret")(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, called, "next handler должен быть вызван при валидном ключе")
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_InvalidKey(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := auth.Middleware("secret")(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "wrong")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.False(t, called, "next handler НЕ должен быть вызван при неверном ключе")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_MissingKey(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := auth.Middleware("secret")(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.False(t, called, "next handler НЕ должен быть вызван при отсутствующем ключе")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_EmptyKey(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := auth.Middleware("secret")(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.False(t, called, "next handler НЕ должен быть вызван при пустом ключе")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
