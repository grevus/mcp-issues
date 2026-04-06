package embed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVoyageEmbedder_HappyPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/embeddings", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body struct {
			Input []string `json:"input"`
			Model string   `json:"model"`
		}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		require.Equal(t, []string{"foo", "bar"}, body.Input)
		require.Equal(t, "voyage-3", body.Model)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": []float32{0.1, 0.2, 0.3, 0.4}},
				{"index": 1, "embedding": []float32{0.5, 0.6, 0.7, 0.8}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	e := NewVoyageEmbedder("test-key", nil)
	e.url = ts.URL + "/v1/embeddings"

	result, err := e.Embed(t.Context(), []string{"foo", "bar"})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.InDeltaSlice(t, []float32{0.1, 0.2, 0.3, 0.4}, result[0], 1e-6)
	require.InDeltaSlice(t, []float32{0.5, 0.6, 0.7, 0.8}, result[1], 1e-6)
}

func TestVoyageEmbedder_Name(t *testing.T) {
	e := NewVoyageEmbedder("key", nil)
	require.Equal(t, "voyage", e.Name())
}

func TestVoyageEmbedder_Dimension(t *testing.T) {
	e := NewVoyageEmbedder("key", nil)
	require.Equal(t, 1024, e.Dimension())
}

func TestVoyageEmbedder_EmptyInput(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
	}))
	defer ts.Close()

	e := NewVoyageEmbedder("test-key", nil)
	e.url = ts.URL + "/v1/embeddings"

	result, err := e.Embed(t.Context(), nil)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 0, requestCount)
}

func TestVoyageEmbedder_HTTPError401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	e := NewVoyageEmbedder("bad-key", nil)
	e.url = ts.URL + "/v1/embeddings"

	_, err := e.Embed(t.Context(), []string{"hello"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "voyage")
	require.Contains(t, err.Error(), "401")
}

func TestVoyageEmbedder_HTTPError500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	e := NewVoyageEmbedder("test-key", nil)
	e.url = ts.URL + "/v1/embeddings"

	_, err := e.Embed(t.Context(), []string{"hello"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "voyage")
	require.Contains(t, err.Error(), "500")
}

func TestVoyageEmbedder_Batching(t *testing.T) {
	var requestCount int
	var requestSizes []int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input []string `json:"input"`
			Model string   `json:"model"`
		}
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		requestCount++
		requestSizes = append(requestSizes, len(body.Input))

		// Для каждого текста возвращаем embedding, где [0] == локальный индекс в batch.
		data := make([]map[string]any, len(body.Input))
		for i := range body.Input {
			data[i] = map[string]any{
				"index":     i,
				"embedding": []float32{float32(i), 0, 0},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer ts.Close()

	// Формируем 200 текстов.
	texts := make([]string, 200)
	for i := range texts {
		texts[i] = fmt.Sprintf("text-%d", i)
	}

	e := NewVoyageEmbedder("test-key", nil)
	e.url = ts.URL + "/v1/embeddings"

	result, err := e.Embed(t.Context(), texts)
	require.NoError(t, err)

	// Должно быть ровно 2 запроса: 128 + 72.
	require.Equal(t, 2, requestCount)
	require.Equal(t, []int{128, 72}, requestSizes)

	// Длина результата == 200.
	require.Len(t, result, 200)

	// Порядок: result[i][0] == локальный индекс внутри batch.
	require.Equal(t, float32(0), result[0][0])
	require.Equal(t, float32(127), result[127][0])
	// Второй batch начинается заново с 0.
	require.Equal(t, float32(0), result[128][0])
	require.Equal(t, float32(71), result[199][0])
}
