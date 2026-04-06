package handlers_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/jira"
	"github.com/stretchr/testify/require"
)

type fakeIssueLister struct {
	gotParams jira.ListIssuesParams
	issues    []jira.Issue
	err       error
}

func (f *fakeIssueLister) ListIssues(_ context.Context, p jira.ListIssuesParams) ([]jira.Issue, error) {
	f.gotParams = p
	return f.issues, f.err
}

func TestListIssues_HappyPath(t *testing.T) {
	fake := &fakeIssueLister{
		issues: []jira.Issue{
			{Key: "ABC-1", Summary: "First issue", Status: "Done"},
			{Key: "ABC-2", Summary: "Second issue", Status: "Done", Assignee: "Alice"},
		},
	}

	h := handlers.ListIssues(fake)
	out, err := h(context.Background(), handlers.ListIssuesInput{
		ProjectKey: "ABC",
		Status:     "Done",
		AssignedTo: "Alice",
		Limit:      10,
	})

	require.NoError(t, err)
	require.Len(t, out.Issues, 2)
	require.Equal(t, "ABC", fake.gotParams.ProjectKey)
	require.Equal(t, "Alice", fake.gotParams.Assignee)
	require.Equal(t, "Done", fake.gotParams.Status)
	require.Equal(t, 10, fake.gotParams.Limit)
}

func TestListIssues_PropagatesError(t *testing.T) {
	fake := &fakeIssueLister{
		err: errors.New("boom"),
	}

	h := handlers.ListIssues(fake)
	_, err := h(context.Background(), handlers.ListIssuesInput{ProjectKey: "ABC"})

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "boom"))
}
