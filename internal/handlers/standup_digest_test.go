package handlers_test

import (
	"context"
	"testing"

	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/tracker"
	"github.com/stretchr/testify/require"
)

func TestStandupDigest_GroupsByStatus(t *testing.T) {
	fake := &fakeIssueLister{
		issues: []tracker.Issue{
			{Key: "ABC-1", Summary: "Ship feature X", Status: "Done"},
			{Key: "ABC-2", Summary: "Refactor Y", Status: "In Progress"},
			{Key: "ABC-3", Summary: "CI flaky", Status: "Blocked"},
			{Key: "ABC-4", Summary: "Plan Z", Status: "To Do"},
		},
	}
	h := handlers.StandupDigest(fake)
	out, err := h(context.Background(), handlers.StandupDigestInput{
		TeamKey: "ABC",
		From:    "2026-04-07",
		To:      "2026-04-08",
	})
	require.NoError(t, err)
	require.Contains(t, out.YesterdaySummary, "ABC-1")
	require.Contains(t, out.TodayFocus, "ABC-2")
	require.Len(t, out.Blockers, 1)
	require.Equal(t, "ABC-3", out.Blockers[0].Key)
	require.Len(t, out.NotableChanges, 1)
	require.Contains(t, out.NotableChanges[0], "ABC-4")

	require.Equal(t, "2026-04-07", fake.gotParams.UpdatedFrom)
	require.Equal(t, "2026-04-08", fake.gotParams.UpdatedTo)
	require.Equal(t, "ABC", fake.gotParams.ProjectKey)
}

func TestStandupDigest_RejectsBadInput(t *testing.T) {
	h := handlers.StandupDigest(&fakeIssueLister{})
	_, err := h(context.Background(), handlers.StandupDigestInput{From: "2026-04-07", To: "2026-04-08"})
	require.Error(t, err)
	_, err = h(context.Background(), handlers.StandupDigestInput{TeamKey: "ABC", From: "2026-04-07"})
	require.Error(t, err)
}
