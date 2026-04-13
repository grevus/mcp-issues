//go:build !(cgo && ORT)

package embed

import (
	"context"
	"fmt"
)

// NewONNXEmbedder заглушка для сборки без тегов cgo+ORT.
// При попытке использовать возвращает ошибку с инструкцией по сборке.
func NewONNXEmbedder(_, _ string) (*ONNXEmbedder, error) {
	return nil, fmt.Errorf("onnx embedder is not available: rebuild with -tags ORT (requires CGO)")
}

// ONNXEmbedder заглушка — тип должен существовать для компиляции switch в cmd/*.
type ONNXEmbedder struct{}

func (e *ONNXEmbedder) Name() string                                        { return "onnx" }
func (e *ONNXEmbedder) Dimension() int                                      { return 0 }
func (e *ONNXEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return nil, fmt.Errorf("onnx embedder stub")
}
