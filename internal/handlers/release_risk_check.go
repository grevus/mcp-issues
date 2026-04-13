package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/tracker"
)

// ReleaseRiskCheckInput — параметры MCP tool release_risk_check.
type ReleaseRiskCheckInput struct {
	ProjectKey       string   `json:"project_key"`
	FixVersion       string   `json:"fix_version"`
	ServicesInvolved []string `json:"services_involved,omitempty"`
	TopK             int      `json:"top_k,omitempty"`
}

// ReleaseRiskCheckOutput — результат release_risk_check.
type ReleaseRiskCheckOutput struct {
	FixVersion         string          `json:"fix_version"`
	OpenIssues         []tracker.Issue `json:"open_issues"`
	BlockedIssues      []tracker.Issue `json:"blocked_issues"`
	RelatedPostmortems []knowledge.Hit `json:"related_postmortems"`
	RiskLevel          string          `json:"risk_level"`
	MissingRunbooks    []string        `json:"missing_runbooks"`
	Summary            string          `json:"summary"`
}

// ReleaseRiskCheck собирает риск-картину по fixVersion: открытые/заблокированные
// задачи из Jira + семантически близкие постмортемы из RAG.
func ReleaseRiskCheck(l IssueLister, r KnowledgeRetriever) Handler[ReleaseRiskCheckInput, ReleaseRiskCheckOutput] {
	return func(ctx context.Context, in ReleaseRiskCheckInput) (ReleaseRiskCheckOutput, error) {
		if in.FixVersion == "" {
			return ReleaseRiskCheckOutput{}, fmt.Errorf("release_risk_check: fix_version is required")
		}
		if in.ProjectKey == "" {
			return ReleaseRiskCheckOutput{}, fmt.Errorf("release_risk_check: project_key is required")
		}
		topK := in.TopK
		if topK > 20 {
			return ReleaseRiskCheckOutput{}, fmt.Errorf("release_risk_check: top_k must be <= 20, got %d", topK)
		}
		if topK <= 0 {
			topK = 5
		}

		issues, err := l.ListIssues(ctx, tracker.ListParams{
			ProjectKey: in.ProjectKey,
			FixVersion: in.FixVersion,
			Limit:      100,
		})
		if err != nil {
			return ReleaseRiskCheckOutput{}, err
		}

		open := make([]tracker.Issue, 0)
		blocked := make([]tracker.Issue, 0)
		for _, is := range issues {
			status := strings.ToLower(is.Status)
			switch {
			case strings.Contains(status, "block"):
				blocked = append(blocked, is)
			case isDoneStatus(status):
				// skip
			default:
				open = append(open, is)
			}
		}

		query := "postmortem incident " + in.FixVersion
		if len(in.ServicesInvolved) > 0 {
			query += " " + strings.Join(in.ServicesInvolved, " ")
		}
		hits, err := r.Search(ctx, in.ProjectKey, query, topK)
		if err != nil {
			return ReleaseRiskCheckOutput{}, err
		}

		risk := computeReleaseRisk(len(open), len(blocked), len(hits))
		summary := fmt.Sprintf(
			"Release %s: %d open, %d blocked, %d related postmortems. Risk: %s.",
			in.FixVersion, len(open), len(blocked), len(hits), risk,
		)

		return ReleaseRiskCheckOutput{
			FixVersion:         in.FixVersion,
			OpenIssues:         open,
			BlockedIssues:      blocked,
			RelatedPostmortems: hits,
			RiskLevel:          risk,
			MissingRunbooks:    []string{},
			Summary:            summary,
		}, nil
	}
}

func isDoneStatus(lower string) bool {
	switch lower {
	case "done", "closed", "resolved":
		return true
	}
	return false
}

// computeReleaseRisk — детерминированная оценка риска релиза.
func computeReleaseRisk(openCount, blockedCount, postmortemCount int) string {
	if blockedCount >= 3 || postmortemCount >= 3 {
		return "high"
	}
	if blockedCount >= 1 || openCount >= 10 {
		return "medium"
	}
	return "low"
}
