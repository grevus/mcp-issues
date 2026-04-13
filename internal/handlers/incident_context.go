package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/tracker"
)

// CommentFetcher — узкий интерфейс: список комментариев задачи в виде plain text.
type CommentFetcher interface {
	GetIssueComments(ctx context.Context, issueKey string) ([]string, error)
}

// IncidentContextInput — параметры MCP tool incident_context.
type IncidentContextInput struct {
	IssueKey   string `json:"issue_key"`
	ProjectKey string `json:"project_key"`
	TopK       int    `json:"top_k,omitempty"`
}

// IncidentContextOutput — детерминированная сводка по инциденту:
// похожие задачи + извлечённые из описания/комментариев гипотезы причин
// и рекомендуемые проверки. DocsLinks зарезервирован под Phase 3 (Confluence)
// и в Phase 2 всегда пустой slice.
type IncidentContextOutput struct {
	Source            tracker.Issue    `json:"source"`
	RelatedIncidents  []knowledge.Hit  `json:"related_incidents"`
	SuspectedCauses   []string         `json:"suspected_causes"`
	RecommendedChecks []string         `json:"recommended_checks"`
	DocsLinks         []string         `json:"docs_links"`
}

// IncidentContext собирает контекст инцидента из Jira-only источников:
// описание + комментарии задачи + RAG-похожие из того же проекта.
// Никакой LLM-генерации внутри — только детерминированные правила.
func IncidentContext(f IssueFetcher, cf CommentFetcher, r KnowledgeRetriever) Handler[IncidentContextInput, IncidentContextOutput] {
	return func(ctx context.Context, in IncidentContextInput) (IncidentContextOutput, error) {
		if in.IssueKey == "" {
			return IncidentContextOutput{}, fmt.Errorf("incident_context: issue_key is required")
		}
		if in.ProjectKey == "" {
			return IncidentContextOutput{}, fmt.Errorf("incident_context: project_key is required")
		}
		topK := in.TopK
		if topK > 20 {
			return IncidentContextOutput{}, fmt.Errorf("incident_context: top_k must be <= 20, got %d", topK)
		}
		if topK <= 0 {
			topK = 5
		}

		src, desc, err := f.GetIssue(ctx, in.IssueKey)
		if err != nil {
			return IncidentContextOutput{}, err
		}

		comments, err := cf.GetIssueComments(ctx, in.IssueKey)
		if err != nil {
			return IncidentContextOutput{}, err
		}

		query := strings.TrimSpace(src.Summary + "\n" + desc)
		hits, err := r.Search(ctx, in.ProjectKey, query, topK+1)
		if err != nil {
			return IncidentContextOutput{}, err
		}
		related := make([]knowledge.Hit, 0, len(hits))
		for _, h := range hits {
			if h.DocKey == in.IssueKey {
				continue
			}
			related = append(related, h)
			if len(related) == topK {
				break
			}
		}

		texts := make([]string, 0, len(comments)+1)
		if strings.TrimSpace(desc) != "" {
			texts = append(texts, desc)
		}
		texts = append(texts, comments...)

		causeKeywords := []string{"caused by", "root cause", "due to", "because of"}
		checkKeywords := []string{"check", "verify", "rollback", "restart", "monitor"}

		causes := extractSentences(texts, causeKeywords, 5)
		checks := extractSentences(texts, checkKeywords, 5)

		return IncidentContextOutput{
			Source:            src,
			RelatedIncidents:  related,
			SuspectedCauses:   causes,
			RecommendedChecks: checks,
			DocsLinks:         []string{},
		}, nil
	}
}

// splitSentences разбивает текст на предложения по . ! ? и переводам строк.
func splitSentences(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		s := strings.TrimSpace(f)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// extractSentences ищет в текстах предложения, содержащие любое из keywords
// (case-insensitive), сохраняет порядок появления, дедуплицирует и режет по cap.
func extractSentences(texts []string, keywords []string, cap int) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, cap)
	for _, t := range texts {
		for _, s := range splitSentences(t) {
			low := strings.ToLower(s)
			matched := false
			for _, kw := range keywords {
				if strings.Contains(low, kw) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
			if len(out) == cap {
				return out
			}
		}
	}
	return out
}
