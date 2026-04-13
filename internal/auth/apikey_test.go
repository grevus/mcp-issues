package auth_test

import (
	"net/http"
	"net/http/httptest"
	"os"
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

// --- MultiKeyMiddleware ---

func TestMultiKeyMiddleware_FirstKey(t *testing.T) {
	keys := []auth.Key{
		{Value: "key-alice", Name: "Alice"},
		{Value: "key-bob", Name: "Bob"},
	}

	var gotName string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = auth.KeyNameFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.MultiKeyMiddleware(keys)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "key-alice")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "Alice", gotName)
}

func TestMultiKeyMiddleware_SecondKey(t *testing.T) {
	keys := []auth.Key{
		{Value: "key-alice", Name: "Alice"},
		{Value: "key-bob", Name: "Bob"},
	}

	var gotName string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = auth.KeyNameFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.MultiKeyMiddleware(keys)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "key-bob")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "Bob", gotName)
}

func TestMultiKeyMiddleware_InvalidKey(t *testing.T) {
	keys := []auth.Key{
		{Value: "key-alice", Name: "Alice"},
		{Value: "key-bob", Name: "Bob"},
	}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := auth.MultiKeyMiddleware(keys)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "key-unknown")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMultiKeyMiddleware_NoKeys(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := auth.MultiKeyMiddleware(nil)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "anything")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- LoadKeys ---

func TestLoadKeys_Valid(t *testing.T) {
	content := `keys:
  - key: "sk-test-1"
    name: "Alice"
  - key: "sk-test-2"
    name: "Bob"
`
	path := t.TempDir() + "/keys.yaml"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	keys, err := auth.LoadKeys(path)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Equal(t, "sk-test-1", keys[0].Value)
	require.Equal(t, "Alice", keys[0].Name)
	require.Equal(t, "sk-test-2", keys[1].Value)
	require.Equal(t, "Bob", keys[1].Name)
}

func TestLoadKeys_EmptyKeyValue(t *testing.T) {
	content := `keys:
  - key: ""
    name: "Alice"
`
	path := t.TempDir() + "/keys.yaml"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := auth.LoadKeys(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty value")
}

func TestLoadKeys_NoKeys(t *testing.T) {
	content := `keys: []`
	path := t.TempDir() + "/keys.yaml"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := auth.LoadKeys(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no keys")
}

func TestLoadKeys_DefaultName(t *testing.T) {
	content := `keys:
  - key: "sk-test-1"
`
	path := t.TempDir() + "/keys.yaml"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	keys, err := auth.LoadKeys(path)
	require.NoError(t, err)
	require.Equal(t, "key-1", keys[0].Name)
}

func TestLoadKeys_FileNotFound(t *testing.T) {
	_, err := auth.LoadKeys("/nonexistent/keys.yaml")
	require.Error(t, err)
}
