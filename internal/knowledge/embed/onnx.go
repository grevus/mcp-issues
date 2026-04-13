//go:build cgo && ORT

package embed

import (
	"context"
	"fmt"

	hugot "github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/options"
	"github.com/knights-analytics/hugot/pipelines"
)

const onnxDimension = 1024

// ONNXEmbedder реализует Embedder через локальный ONNX-inference (hugot + ONNX Runtime).
// Модель загружается из локальной директории; требует libonnxruntime.dylib/.so на машине.
type ONNXEmbedder struct {
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
}

// NewONNXEmbedder создаёт ONNXEmbedder.
//   - modelPath — директория с model.onnx, tokenizer.json, config.json.
//   - libDir — директория с libonnxruntime.dylib/.so (пустая строка = auto-detect).
func NewONNXEmbedder(modelPath, libDir string) (*ONNXEmbedder, error) {
	var opts []options.WithOption
	if libDir != "" {
		opts = append(opts, options.WithOnnxLibraryPath(libDir))
	}

	session, err := hugot.NewORTSession(opts...)
	if err != nil {
		return nil, fmt.Errorf("onnx: create session: %w", err)
	}

	pipeline, err := hugot.NewPipeline(session, hugot.FeatureExtractionConfig{
		ModelPath:    modelPath,
		Name:         "mcp-jira-embed",
		OnnxFilename: "model.onnx",
	})
	if err != nil {
		_ = session.Destroy()
		return nil, fmt.Errorf("onnx: create pipeline: %w", err)
	}

	return &ONNXEmbedder{session: session, pipeline: pipeline}, nil
}

// Name возвращает "onnx".
func (e *ONNXEmbedder) Name() string { return "onnx" }

// Dimension возвращает 1024 (BAAI/bge-large-en-v1.5).
func (e *ONNXEmbedder) Dimension() int { return onnxDimension }

// Embed запускает inference для batch текстов.
// Контракт: длина выхода равна длине входа, порядок сохранён.
func (e *ONNXEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	out, err := e.pipeline.RunPipeline(texts)
	if err != nil {
		return nil, fmt.Errorf("onnx: run pipeline: %w", err)
	}
	return out.Embeddings, nil
}
