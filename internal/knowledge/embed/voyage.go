package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const voyageDefaultURL = "https://api.voyageai.com/v1/embeddings"
const voyageModel = "voyage-3"
const voyageDimension = 1024
const voyageBatchSize = 128

// VoyageEmbedder реализует Embedder через Voyage AI API.
type VoyageEmbedder struct {
	apiKey string
	url    string // переопределяется в тестах
	http   *http.Client
}

// NewVoyageEmbedder создаёт клиент.
// Если httpClient == nil — используется http.DefaultClient.
func NewVoyageEmbedder(apiKey string, httpClient *http.Client) *VoyageEmbedder {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &VoyageEmbedder{
		apiKey: apiKey,
		url:    voyageDefaultURL,
		http:   httpClient,
	}
}

// Name возвращает "voyage".
func (e *VoyageEmbedder) Name() string { return "voyage" }

// Dimension возвращает фиксированную размерность векторов (1024),
// согласованную со схемой Postgres vector(1024) из spec §5.3.
func (e *VoyageEmbedder) Dimension() int { return voyageDimension }

// voyageRequest — тело запроса к Voyage AI.
type voyageRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

// voyageEmbeddingItem — один элемент из data[].
type voyageEmbeddingItem struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// voyageResponse — ответ Voyage AI.
type voyageResponse struct {
	Data []voyageEmbeddingItem `json:"data"`
}

// Embed расщепляет texts на batch-и по voyageBatchSize, выполняет N HTTP-запросов
// и склеивает результаты в порядке оригинального ввода.
func (e *VoyageEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	result := make([][]float32, 0, len(texts))
	for i := 0; i < len(texts); i += voyageBatchSize {
		end := i + voyageBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		chunk := texts[i:end]

		embeddings, err := e.embedBatch(ctx, chunk)
		if err != nil {
			return nil, err
		}
		result = append(result, embeddings...)
	}

	return result, nil
}

// embedBatch выполняет один HTTP-запрос к Voyage AI для заданного chunk-а.
// Порядок результатов гарантирован через поле index из ответа.
func (e *VoyageEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	payload := voyageRequest{
		Input: texts,
		Model: voyageModel,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("voyage: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("voyage: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voyage: http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("voyage: POST %s -> %d", e.url, resp.StatusCode)
	}

	var voyageResp voyageResponse
	if err := json.NewDecoder(resp.Body).Decode(&voyageResp); err != nil {
		return nil, fmt.Errorf("voyage: decode response: %w", err)
	}

	// Восстанавливаем порядок по полю index из ответа.
	result := make([][]float32, len(texts))
	for _, item := range voyageResp.Data {
		result[item.Index] = item.Embedding
	}

	return result, nil
}
