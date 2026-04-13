package handlers_test

import (
	"context"
	"testing"

	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/tracker"
	"github.com/stretchr/testify/require"
)

type fakeSprintReporter struct {
	rep tracker.SprintReport
	err error
}

func (f *fakeSprintReporter) GetSprintReport(_ context.Context, _ int, _ int) (tracker.SprintReport, error) {
	return f.rep, f.err
}

func TestSprintHealthReport_HighRiskAndActions(t *testing.T) {
	f := &fakeSprintReporter{
		rep: tracker.SprintReport{
			Health: tracker.SprintHealth{
				BoardID: 1, SprintName: "Sprint 42",
				Total: 10, Done: 3, InProgress: 4, Blocked: 3, Velocity: 8,
			},
			BlockedIssues: []tracker.Issue{
				{Key: "ABC-1", Summary: "DB slow"},
				{Key: "ABC-2", Summary: "Auth down"},
			},
		},
	}
	h := handlers.SprintHealthReport(f, nil)
	out, err := h(context.Background(), handlers.SprintHealthReportInput{BoardID: 1})
	require.NoError(t, err)
	require.Equal(t, handlers.RiskHigh, out.RiskLevel)
	require.Len(t, out.ActionItems, 2)
	require.Contains(t, out.Summary, "Sprint 42")
	require.Contains(t, out.ActionItems[0], "ABC-1")
}

func TestSprintHealthReport_LowRisk(t *testing.T) {
	f := &fakeSprintReporter{
		rep: tracker.SprintReport{
			Health: tracker.SprintHealth{Total: 10, Done: 7, InProgress: 3, Blocked: 0},
		},
	}
	h := handlers.SprintHealthReport(f, nil)
	out, err := h(context.Background(), handlers.SprintHealthReportInput{BoardID: 1})
	require.NoError(t, err)
	require.Equal(t, handlers.RiskLow, out.RiskLevel)
}

func TestSprintHealthReport_RejectsBadInput(t *testing.T) {
	h := handlers.SprintHealthReport(&fakeSprintReporter{}, nil)
	_, err := h(context.Background(), handlers.SprintHealthReportInput{BoardID: 0})
	require.Error(t, err)
}
