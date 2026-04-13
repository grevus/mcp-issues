package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/tracker"
)

// TicketTriageInput — параметры MCP tool ticket_triage (Phase 2).
type TicketTriageInput struct {
	IssueKey   string `json:"issue_key"`
	ProjectKey string `json:"project_key"`
	TopK       int    `json:"top_k,omitempty"`
}

// TicketTriageOutput — результат ticket_triage.
type TicketTriageOutput struct {
	Source         tracker.Issue   `json:"source"`
	SuggestedTeam  string          `json:"suggested_team"`
	Priority       string          `json:"priority"`
	Rationale      string          `json:"rationale"`
	SimilarIssues  []knowledge.Hit `json:"similar_issues"`
}

// priorityRule — одна запись в детерминированной таблице эвристик.
type priorityRule struct {
	level    string
	keywords []string
}

// priorityRules упорядочены от высокого приоритета к низкому.
// Первый матч выигрывает.
var priorityRules = []priorityRule{
	{level: "high", keywords: []string{"outage", "prod down", "sev1", "sev-1", "blocker", "critical", "p0"}},
	{level: "medium", keywords: []string{"prod", "production", "customer", "regression", "p1"}},
}

// guessPriority — детерминированное определение приоритета по keyword-эвристике.
// Вход ожидается уже нормализованным (lowercase).
func guessPriority(text string) string {
	for _, r := range priorityRules {
		for _, kw := range r.keywords {
			if strings.Contains(text, kw) {
				return r.level
			}
		}
	}
	return "low"
}

// extractAssigneeFromExcerpt пытается достать имя assignee из rendered-excerpt
// (см. internal/rag/index/render.go: строка "Assignee: <name>").
// Возвращает пустую строку, если поля нет.
func extractAssigneeFromExcerpt(excerpt string) string {
	for _, line := range strings.Split(excerpt, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Assignee:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Assignee:"))
		}
	}
	return ""
}

// TicketTriage реализует MCP tool ticket_triage: по заданной задаче подбирает
// похожие через RAG, предполагает команду (по assignee-частоте в похожих) и
// приоритет (по keyword-эвристике).
func TicketTriage(f IssueFetcher, r KnowledgeRetriever) Handler[TicketTriageInput, TicketTriageOutput] {
	return func(ctx context.Context, in TicketTriageInput) (TicketTriageOutput, error) {
		if in.IssueKey == "" {
			return TicketTriageOutput{}, fmt.Errorf("ticket_triage: issue_key is required")
		}
		if in.ProjectKey == "" {
			return TicketTriageOutput{}, fmt.Errorf("ticket_triage: project_key is required")
		}
		topK := in.TopK
		if topK > 20 {
			return TicketTriageOutput{}, fmt.Errorf("ticket_triage: top_k must be <= 20, got %d", topK)
		}
		if topK <= 0 {
			topK = 10
		}

		src, desc, err := f.GetIssue(ctx, in.IssueKey)
		if err != nil {
			return TicketTriageOutput{}, err
		}

		query := strings.TrimSpace(src.Summary + "\n" + desc)
		hits, err := r.Search(ctx, in.ProjectKey, query, topK+1)
		if err != nil {
			return TicketTriageOutput{}, err
		}

		similar := make([]knowledge.Hit, 0, len(hits))
		for _, h := range hits {
			if h.DocKey == in.IssueKey {
				continue
			}
			similar = append(similar, h)
			if len(similar) == topK {
				break
			}
		}

		team := inferTeam(similar)
		priority := guessPriority(strings.ToLower(src.Summary + " " + desc))

		var rationale string
		if team == "" {
			rationale = fmt.Sprintf("Team not inferred (no assignees in similar issues); priority %q based on keywords", priority)
		} else {
			rationale = fmt.Sprintf("Team %q inferred from %d similar issues; priority %q based on keywords", team, len(similar), priority)
		}

		return TicketTriageOutput{
			Source:        src,
			SuggestedTeam: team,
			Priority:      priority,
			Rationale:     rationale,
			SimilarIssues: similar,
		}, nil
	}
}

// inferTeam возвращает самое частое непустое имя assignee из similar.
// Ties разрешаются порядком первого появления.
func inferTeam(similar []knowledge.Hit) string {
	counts := make(map[string]int)
	order := make([]string, 0)
	for _, h := range similar {
		name := extractAssigneeFromExcerpt(h.Excerpt)
		if name == "" {
			continue
		}
		if _, ok := counts[name]; !ok {
			order = append(order, name)
		}
		counts[name]++
	}
	best := ""
	bestCount := 0
	for _, name := range order {
		if counts[name] > bestCount {
			best = name
			bestCount = counts[name]
		}
	}
	return best
}
