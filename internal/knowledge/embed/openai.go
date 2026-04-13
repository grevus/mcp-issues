package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const openAIDefaultURL = "https://api.openai.com/v1/embeddings"
const openAIModel = "text-embedding-3-small"
const openAIDimension = 1024
const openaiBatchSize = 100

// OpenAIEmbedder реализует Embedder через OpenAI Embeddings API.
type OpenAIEmbedder struct {
	apiKey  string
	baseURL string // переопределяется в тестах
	http    *http.Client
}

// NewOpenAIEmbedder создаёт клиент.
// Если httpClient == nil — используется http.DefaultClient.
func NewOpenAIEmbedder(apiKey string, httpClient *http.Client) *OpenAIEmbedder {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OpenAIEmbedder{
		apiKey:  apiKey,
		baseURL: openAIDefaultURL,
		http:    httpClient,
	}
}

// Name возвращает "openai".
func (e *OpenAIEmbedder) Name() string { return "openai" }

// Dimension возвращает фиксированную размерность векторов (1024),
// согласованную со схемой Postgres vector(1024) из spec §5.3.
func (e *OpenAIEmbedder) Dimension() int { return openAIDimension }

// openAIRequest — тело запроса к OpenAI Embeddings API.
type openAIRequest struct {
	Input      []string `json:"input"`
	Model      string   `json:"model"`
	Dimensions int      `json:"dimensions"`
}

// openAIEmbeddingItem — один элемент из data[].
type openAIEmbeddingItem struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// openAIResponse — ответ OpenAI Embeddings API.
type openAIResponse struct {
	Data []openAIEmbeddingItem `json:"data"`
}

// Embed расщепляет texts на батчи по openaiBatchSize, выполняет N HTTP-запросов
// и склеивает результаты в порядке оригинального ввода.
func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	result := make([][]float32, 0, len(texts))
	for i := 0; i < len(texts); i += openaiBatchSize {
		end := i + openaiBatchSize
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

// embedBatch выполняет один HTTP-запрос к OpenAI для заданного chunk-а.
// Порядок результатов гарантирован через поле index из ответа.
func (e *OpenAIEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	payload := openAIRequest{
		Input:      texts,
		Model:      openAIModel,
		Dimensions: openAIDimension,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openai: POST %s -> %d", e.baseURL, resp.StatusCode)
	}

	var openAIResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("openai: decode response: %w", err)
	}

	// Восстанавливаем порядок по полю index из ответа.
	result := make([][]float32, len(texts))
	for _, item := range openAIResp.Data {
		result[item.Index] = item.Embedding
	}

	return result, nil
}
