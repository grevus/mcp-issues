package handlers

import (
	"context"
	"fmt"
)

// Hit — один результат semantic-поиска. Дублирует форму store.Hit (Task 32),
// чтобы handlers не зависел от RAG-пакетов на этапе Phase 5.
type Hit struct {
	IssueKey string  `json:"issue_key"`
	Summary  string  `json:"summary"`
	Status   string  `json:"status"`
	Score    float32 `json:"score"`
	Excerpt  string  `json:"excerpt"`
}

// KnowledgeRetriever — узкий интерфейс для handler SearchKnowledge.
type KnowledgeRetriever interface {
	Search(ctx context.Context, projectKey, query string, topK int) ([]Hit, error)
}

// SearchKnowledgeInput — параметры MCP tool search_jira_knowledge.
type SearchKnowledgeInput struct {
	ProjectKey string `json:"project_key"`
	Query      string `json:"query"`
	TopK       int    `json:"top_k,omitempty"`
}

// SearchKnowledgeOutput — результат MCP tool search_jira_knowledge.
type SearchKnowledgeOutput struct {
	Hits []Hit `json:"hits"`
}

// SearchKnowledge возвращает Handler с валидацией поля TopK (Task 25).
// TopK <= 0 → default 5; 1–20 → as-is; > 20 → ошибка без вызова retriever.
func SearchKnowledge(r KnowledgeRetriever) Handler[SearchKnowledgeInput, SearchKnowledgeOutput] {
	return func(ctx context.Context, in SearchKnowledgeInput) (SearchKnowledgeOutput, error) {
		topK := in.TopK
		if topK > 20 {
			return SearchKnowledgeOutput{}, fmt.Errorf("search_jira_knowledge: top_k must be <= 20, got %d", topK)
		}
		if topK <= 0 {
			topK = 5
		}
		hits, err := r.Search(ctx, in.ProjectKey, in.Query, topK)
		if err != nil {
			return SearchKnowledgeOutput{}, err
		}
		return SearchKnowledgeOutput{Hits: hits}, nil
	}
}
