package embed

import "context"

// Embedder — контракт реализации embeddings.
// Реализации (Voyage, OpenAI) делают batching внутри.
// Контракт: длина выхода равна длине входа, порядок сохранён.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int // 1024 для всех текущих реализаций
	Name() string   // "voyage" / "openai" — для логов
}
