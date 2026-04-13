package embed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIEmbedder_HappyPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/embeddings", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body struct {
			Input      []string `json:"input"`
			Model      string   `json:"model"`
			Dimensions int      `json:"dimensions"`
		}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		require.Equal(t, []string{"hello", "world"}, body.Input)
		require.Equal(t, "text-embedding-3-small", body.Model)
		require.Equal(t, 1024, body.Dimensions)

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

	e := NewOpenAIEmbedder("test-key", nil)
	e.baseURL = ts.URL + "/v1/embeddings"

	result, err := e.Embed(t.Context(), []string{"hello", "world"})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.InDeltaSlice(t, []float32{0.1, 0.2, 0.3, 0.4}, result[0], 1e-6)
	require.InDeltaSlice(t, []float32{0.5, 0.6, 0.7, 0.8}, result[1], 1e-6)
}

func TestOpenAIEmbedder_Name(t *testing.T) {
	e := NewOpenAIEmbedder("key", nil)
	require.Equal(t, "openai", e.Name())
}

func TestOpenAIEmbedder_Dimension(t *testing.T) {
	e := NewOpenAIEmbedder("key", nil)
	require.Equal(t, 1024, e.Dimension())
}

func TestOpenAIEmbedder_Batching(t *testing.T) {
	// 250 текстов → 3 батча: 100 + 100 + 50
	const total = 250

	var requestCount int
	var receivedBatchSizes []int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input []string `json:"input"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		batchIdx := requestCount
		receivedBatchSizes = append(receivedBatchSizes, len(body.Input))
		requestCount++

		// Формируем ответ: embedding для каждого текста в батче.
		// Каждый вектор кодирует глобальный индекс: [float32(globalIdx), 0, 0, 0].
		startGlobal := batchIdx * 100
		items := make([]map[string]any, len(body.Input))
		// Возвращаем в обратном порядке, чтобы проверить сортировку по data[].index.
		for localIdx := range body.Input {
			globalIdx := startGlobal + localIdx
			items[len(body.Input)-1-localIdx] = map[string]any{
				"index":     localIdx,
				"embedding": []float32{float32(globalIdx), 0, 0, 0},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
	}))
	defer ts.Close()

	texts := make([]string, total)
	for i := range texts {
		texts[i] = fmt.Sprintf("text-%d", i)
	}

	e := NewOpenAIEmbedder("test-key", nil)
	e.baseURL = ts.URL + "/v1/embeddings"

	result, err := e.Embed(t.Context(), texts)
	require.NoError(t, err)
	require.Equal(t, 3, requestCount, "должно быть 3 HTTP-запроса")
	require.Equal(t, []int{100, 100, 50}, receivedBatchSizes)
	require.Len(t, result, total)

	// Проверяем порядок: result[i][0] должен быть float32(i).
	for i, vec := range result {
		require.InDelta(t, float32(i), vec[0], 1e-6, "неверный порядок на позиции %d", i)
	}
}

func TestOpenAIEmbedder_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	e := NewOpenAIEmbedder("test-key", nil)
	e.baseURL = ts.URL + "/v1/embeddings"

	_, err := e.Embed(t.Context(), []string{"hello"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "openai: POST")
	require.Contains(t, err.Error(), "500")
}
