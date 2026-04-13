package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/tracker"
)

// IssueFetcher — узкий интерфейс: получить одну задачу по ключу вместе с её описанием.
type IssueFetcher interface {
	GetIssue(ctx context.Context, key string) (tracker.Issue, string, error)
}

// SimilarIssuesInput — параметры MCP tool similar_issues.
type SimilarIssuesInput struct {
	ProjectKey string `json:"project_key"`
	IssueKey   string `json:"issue_key"`
	TopK       int    `json:"top_k,omitempty"`
}

// SimilarIssuesOutput — результат similar_issues.
// SimilarityScore агрегируется как score лучшего hit.
type SimilarIssuesOutput struct {
	Source        tracker.Issue    `json:"source"`
	SimilarIssues []knowledge.Hit  `json:"similar_issues"`
}

// SimilarIssues использует RAG retriever: берёт summary + description задачи
// как запрос, отфильтровывая саму задачу из результатов.
func SimilarIssues(f IssueFetcher, r KnowledgeRetriever) Handler[SimilarIssuesInput, SimilarIssuesOutput] {
	return func(ctx context.Context, in SimilarIssuesInput) (SimilarIssuesOutput, error) {
		if in.IssueKey == "" {
			return SimilarIssuesOutput{}, fmt.Errorf("similar_issues: issue_key is required")
		}
		if in.ProjectKey == "" {
			return SimilarIssuesOutput{}, fmt.Errorf("similar_issues: project_key is required")
		}
		topK := in.TopK
		if topK > 20 {
			return SimilarIssuesOutput{}, fmt.Errorf("similar_issues: top_k must be <= 20, got %d", topK)
		}
		if topK <= 0 {
			topK = 5
		}

		src, desc, err := f.GetIssue(ctx, in.IssueKey)
		if err != nil {
			return SimilarIssuesOutput{}, err
		}

		query := strings.TrimSpace(src.Summary + "\n" + desc)
		// +1 на случай self-match в топе.
		hits, err := r.Search(ctx, in.ProjectKey, query, topK+1)
		if err != nil {
			return SimilarIssuesOutput{}, err
		}

		filtered := make([]knowledge.Hit, 0, len(hits))
		for _, h := range hits {
			if h.DocKey == in.IssueKey {
				continue
			}
			filtered = append(filtered, h)
			if len(filtered) == topK {
				break
			}
		}

		return SimilarIssuesOutput{Source: src, SimilarIssues: filtered}, nil
	}
}
