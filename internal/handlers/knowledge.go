package handlers

import "context"

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

// SearchKnowledge возвращает Handler. Валидация top_k — Task 25.
func SearchKnowledge(r KnowledgeRetriever) Handler[SearchKnowledgeInput, SearchKnowledgeOutput] {
	return func(ctx context.Context, in SearchKnowledgeInput) (SearchKnowledgeOutput, error) {
		hits, err := r.Search(ctx, in.ProjectKey, in.Query, in.TopK)
		if err != nil {
			return SearchKnowledgeOutput{}, err
		}
		return SearchKnowledgeOutput{Hits: hits}, nil
	}
}
