package handlers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/tracker"
	"github.com/stretchr/testify/require"
)

type fakeTriageFetcher struct {
	issue tracker.Issue
	desc  string
	err   error
}

func (f *fakeTriageFetcher) GetIssue(_ context.Context, _ string) (tracker.Issue, string, error) {
	return f.issue, f.desc, f.err
}

type fakeTriageRetriever struct {
	hits []knowledge.Hit
	err  error
}

func (f *fakeTriageRetriever) Search(_ context.Context, _ string, _ string, _ int) ([]knowledge.Hit, error) {
	return f.hits, f.err
}

func excerptWithAssignee(name string) string {
	return "[KEY] ABC-X\nSummary: x\nStatus: Open\nAssignee: " + name + "\n\nDescription:\n..."
}

func TestTicketTriage_PriorityHeuristic(t *testing.T) {
	cases := []struct {
		name    string
		summary string
		desc    string
		want    string
	}{
		{"sev1", "SEV1 outage in checkout", "", "high"},
		{"p0_keyword", "Urgent p0 incident", "", "high"},
		{"blocker", "Blocker bug in login", "", "high"},
		{"critical", "Critical failure", "", "high"},
		{"prod_down", "prod down since 10am", "", "high"},
		{"production_medium", "Production issue on checkout", "", "medium"},
		{"customer", "Customer cannot pay", "", "medium"},
		{"regression", "Regression in search", "", "medium"},
		{"p1", "p1 bug in reports", "", "medium"},
		{"low", "Typo in tooltip", "cosmetic change", "low"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeTriageFetcher{issue: tracker.Issue{Key: "ABC-1", Summary: tc.summary}, desc: tc.desc}
			r := &fakeTriageRetriever{}
			h := handlers.TicketTriage(f, r)
			out, err := h(context.Background(), handlers.TicketTriageInput{IssueKey: "ABC-1", ProjectKey: "ABC"})
			require.NoError(t, err)
			require.Equal(t, tc.want, out.Priority)
		})
	}
}

func TestTicketTriage_TeamFrequency(t *testing.T) {
	f := &fakeTriageFetcher{issue: tracker.Issue{Key: "ABC-1", Summary: "x"}}
	r := &fakeTriageRetriever{hits: []knowledge.Hit{
		{DocKey: "ABC-2", Excerpt: excerptWithAssignee("Alice")},
		{DocKey: "ABC-3", Excerpt: excerptWithAssignee("Alice")},
		{DocKey: "ABC-4", Excerpt: excerptWithAssignee("Bob")},
	}}
	h := handlers.TicketTriage(f, r)
	out, err := h(context.Background(), handlers.TicketTriageInput{IssueKey: "ABC-1", ProjectKey: "ABC"})
	require.NoError(t, err)
	require.Equal(t, "Alice", out.SuggestedTeam)
	require.Contains(t, out.Rationale, "Alice")
}

func TestTicketTriage_TeamTieFirstAppearance(t *testing.T) {
	f := &fakeTriageFetcher{issue: tracker.Issue{Key: "ABC-1", Summary: "x"}}
	r := &fakeTriageRetriever{hits: []knowledge.Hit{
		{DocKey: "ABC-2", Excerpt: excerptWithAssignee("Alice")},
		{DocKey: "ABC-3", Excerpt: excerptWithAssignee("Bob")},
	}}
	h := handlers.TicketTriage(f, r)
	out, err := h(context.Background(), handlers.TicketTriageInput{IssueKey: "ABC-1", ProjectKey: "ABC"})
	require.NoError(t, err)
	require.Equal(t, "Alice", out.SuggestedTeam)
}

func TestTicketTriage_NoAssignees(t *testing.T) {
	f := &fakeTriageFetcher{issue: tracker.Issue{Key: "ABC-1", Summary: "x"}}
	r := &fakeTriageRetriever{hits: []knowledge.Hit{
		{DocKey: "ABC-2", Excerpt: "no assignee here"},
	}}
	h := handlers.TicketTriage(f, r)
	out, err := h(context.Background(), handlers.TicketTriageInput{IssueKey: "ABC-1", ProjectKey: "ABC"})
	require.NoError(t, err)
	require.Equal(t, "", out.SuggestedTeam)
	require.Contains(t, out.Rationale, "Team not inferred")
}

func TestTicketTriage_Validation(t *testing.T) {
	h := handlers.TicketTriage(&fakeTriageFetcher{}, &fakeTriageRetriever{})
	_, err := h(context.Background(), handlers.TicketTriageInput{ProjectKey: "ABC"})
	require.Error(t, err)
	_, err = h(context.Background(), handlers.TicketTriageInput{IssueKey: "ABC-1"})
	require.Error(t, err)
	_, err = h(context.Background(), handlers.TicketTriageInput{IssueKey: "ABC-1", ProjectKey: "ABC", TopK: 100})
	require.Error(t, err)
}

func TestTicketTriage_FetcherError(t *testing.T) {
	h := handlers.TicketTriage(&fakeTriageFetcher{err: errors.New("boom")}, &fakeTriageRetriever{})
	_, err := h(context.Background(), handlers.TicketTriageInput{IssueKey: "ABC-1", ProjectKey: "ABC"})
	require.Error(t, err)
}

func TestTicketTriage_FiltersSelfMatch(t *testing.T) {
	f := &fakeTriageFetcher{issue: tracker.Issue{Key: "ABC-1", Summary: "login"}}
	r := &fakeTriageRetriever{hits: []knowledge.Hit{
		{DocKey: "ABC-1", Excerpt: excerptWithAssignee("Self")},
		{DocKey: "ABC-2", Excerpt: excerptWithAssignee("Alice")},
	}}
	h := handlers.TicketTriage(f, r)
	out, err := h(context.Background(), handlers.TicketTriageInput{IssueKey: "ABC-1", ProjectKey: "ABC", TopK: 5})
	require.NoError(t, err)
	require.Len(t, out.SimilarIssues, 1)
	require.Equal(t, "ABC-2", out.SimilarIssues[0].DocKey)
	require.Equal(t, "Alice", out.SuggestedTeam)
}
