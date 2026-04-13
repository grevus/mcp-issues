package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/grevus/mcp-jira/internal/tracker"
)

// StandupDigestInput — параметры MCP tool standup_digest.
// TeamKey маппится в projectKey в JQL.
type StandupDigestInput struct {
	TeamKey string `json:"team_key"`
	From    string `json:"from"` // "YYYY-MM-DD" или "YYYY-MM-DD HH:MM"
	To      string `json:"to"`
	Limit   int    `json:"limit,omitempty"`
}

// StandupDigestOutput — сгруппированный отчёт для async standup.
type StandupDigestOutput struct {
	YesterdaySummary string          `json:"yesterday_summary"`
	TodayFocus       string          `json:"today_focus"`
	Blockers         []tracker.Issue `json:"blockers"`
	NotableChanges   []string        `json:"notable_changes"`
}

// StandupDigest — handler для standup_digest. Использует IssueLister (узкий),
// переиспользуя фильтры UpdatedFrom/UpdatedTo в ListParams.
func StandupDigest(l IssueLister) Handler[StandupDigestInput, StandupDigestOutput] {
	return func(ctx context.Context, in StandupDigestInput) (StandupDigestOutput, error) {
		if in.TeamKey == "" {
			return StandupDigestOutput{}, fmt.Errorf("standup_digest: team_key is required")
		}
		if in.From == "" || in.To == "" {
			return StandupDigestOutput{}, fmt.Errorf("standup_digest: from and to are required")
		}

		issues, err := l.ListIssues(ctx, tracker.ListParams{
			ProjectKey:  in.TeamKey,
			UpdatedFrom: in.From,
			UpdatedTo:   in.To,
			Limit:       in.Limit,
		})
		if err != nil {
			return StandupDigestOutput{}, err
		}

		var (
			done       []string
			inProgress []string
			blockers   []tracker.Issue
			notable    []string
		)
		for _, is := range issues {
			status := strings.ToLower(is.Status)
			line := fmt.Sprintf("%s — %s", is.Key, is.Summary)
			switch {
			case strings.Contains(status, "block"):
				blockers = append(blockers, is)
			case status == "done" || status == "closed" || status == "resolved":
				done = append(done, line)
			case strings.Contains(status, "progress"):
				inProgress = append(inProgress, line)
			default:
				notable = append(notable, line)
			}
		}

		return StandupDigestOutput{
			YesterdaySummary: joinOrNone(done),
			TodayFocus:       joinOrNone(inProgress),
			Blockers:         blockers,
			NotableChanges:   notable,
		}, nil
	}
}

func joinOrNone(lines []string) string {
	if len(lines) == 0 {
		return "(none)"
	}
	return strings.Join(lines, "\n")
}
