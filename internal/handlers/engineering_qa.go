package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/grevus/mcp-jira/internal/knowledge"
)

// EngineeringQAInput — параметры MCP tool engineering_qa.
type EngineeringQAInput struct {
	Question    string `json:"question"`
	ProjectKey  string `json:"project_key"`
	ContextHint string `json:"context_hint,omitempty"`
	TopK        int    `json:"top_k,omitempty"`
}

// EngineeringQAOutput — результат engineering_qa: список цитат из RAG-индекса.
type EngineeringQAOutput struct {
	Citations []knowledge.Hit `json:"citations"`
}

// EngineeringQA отвечает на инженерный вопрос, возвращая релевантные issue из
// RAG-индекса в качестве цитат. LLM-клиент сам формулирует ответ поверх них.
// TODO(phase3): include doc_hits when RAG_DOCS_ENABLED=true.
func EngineeringQA(r KnowledgeRetriever) Handler[EngineeringQAInput, EngineeringQAOutput] {
	return func(ctx context.Context, in EngineeringQAInput) (EngineeringQAOutput, error) {
		if strings.TrimSpace(in.Question) == "" {
			return EngineeringQAOutput{}, fmt.Errorf("engineering_qa: question is required")
		}
		if in.ProjectKey == "" {
			return EngineeringQAOutput{}, fmt.Errorf("engineering_qa: project_key is required")
		}
		topK := in.TopK
		if topK > 20 {
			return EngineeringQAOutput{}, fmt.Errorf("engineering_qa: top_k must be <= 20, got %d", topK)
		}
		if topK <= 0 {
			topK = 5
		}

		query := strings.TrimSpace(in.Question + " " + in.ContextHint)
		hits, err := r.Search(ctx, in.ProjectKey, query, topK)
		if err != nil {
			return EngineeringQAOutput{}, err
		}
		return EngineeringQAOutput{Citations: hits}, nil
	}
}
