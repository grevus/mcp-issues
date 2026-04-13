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

type fakeIncidentFetcher struct {
	issue tracker.Issue
	desc  string
	err   error
}

func (f *fakeIncidentFetcher) GetIssue(_ context.Context, _ string) (tracker.Issue, string, error) {
	return f.issue, f.desc, f.err
}

type fakeComments struct {
	comments []string
	err      error
}

func (f *fakeComments) GetIssueComments(_ context.Context, _ string) ([]string, error) {
	return f.comments, f.err
}

type fakeIncidentRetriever struct {
	hits []knowledge.Hit
	err  error
}

func (f *fakeIncidentRetriever) Search(_ context.Context, _ string, _ string, _ int) ([]knowledge.Hit, error) {
	return f.hits, f.err
}

func TestIncidentContext_HappyPath(t *testing.T) {
	f := &fakeIncidentFetcher{
		issue: tracker.Issue{Key: "ABC-10", Summary: "Checkout 500"},
		desc:  "Production checkout failing. Due to bad deploy of service X.",
	}
	cf := &fakeComments{comments: []string{
		"Root cause was a stale cache entry.",
		"Please verify the redis connection. Also rollback the last release.",
		"Unrelated note about lunch.",
	}}
	r := &fakeIncidentRetriever{hits: []knowledge.Hit{
		{DocKey: "ABC-10", Title: "self"},
		{DocKey: "ABC-7", Title: "prev incident"},
		{DocKey: "ABC-3", Title: "older incident"},
	}}

	h := handlers.IncidentContext(f, cf, r)
	out, err := h(context.Background(), handlers.IncidentContextInput{
		IssueKey: "ABC-10", ProjectKey: "ABC", TopK: 5,
	})
	require.NoError(t, err)
	require.Equal(t, "ABC-10", out.Source.Key)
	require.Len(t, out.RelatedIncidents, 2)
	require.Equal(t, "ABC-7", out.RelatedIncidents[0].DocKey)
	require.NotEmpty(t, out.SuspectedCauses)
	require.Contains(t, out.SuspectedCauses[0], "Due to")
	require.NotEmpty(t, out.RecommendedChecks)
	require.NotNil(t, out.DocsLinks)
	require.Len(t, out.DocsLinks, 0)
}

func TestIncidentContext_Validation(t *testing.T) {
	h := handlers.IncidentContext(&fakeIncidentFetcher{}, &fakeComments{}, &fakeIncidentRetriever{})
	_, err := h(context.Background(), handlers.IncidentContextInput{ProjectKey: "ABC"})
	require.Error(t, err)
	_, err = h(context.Background(), handlers.IncidentContextInput{IssueKey: "ABC-1"})
	require.Error(t, err)
	_, err = h(context.Background(), handlers.IncidentContextInput{IssueKey: "ABC-1", ProjectKey: "ABC", TopK: 21})
	require.Error(t, err)
}

func TestIncidentContext_FetcherError(t *testing.T) {
	f := &fakeIncidentFetcher{err: errors.New("boom")}
	h := handlers.IncidentContext(f, &fakeComments{}, &fakeIncidentRetriever{})
	_, err := h(context.Background(), handlers.IncidentContextInput{IssueKey: "ABC-1", ProjectKey: "ABC"})
	require.Error(t, err)
}

func TestIncidentContext_CommentsError(t *testing.T) {
	f := &fakeIncidentFetcher{issue: tracker.Issue{Key: "ABC-1"}}
	cf := &fakeComments{err: errors.New("nope")}
	h := handlers.IncidentContext(f, cf, &fakeIncidentRetriever{})
	_, err := h(context.Background(), handlers.IncidentContextInput{IssueKey: "ABC-1", ProjectKey: "ABC"})
	require.Error(t, err)
}

func TestIncidentContext_NoComments(t *testing.T) {
	f := &fakeIncidentFetcher{issue: tracker.Issue{Key: "ABC-1", Summary: "s"}, desc: "plain text only"}
	cf := &fakeComments{comments: nil}
	r := &fakeIncidentRetriever{hits: []knowledge.Hit{{DocKey: "ABC-2"}}}
	h := handlers.IncidentContext(f, cf, r)
	out, err := h(context.Background(), handlers.IncidentContextInput{IssueKey: "ABC-1", ProjectKey: "ABC"})
	require.NoError(t, err)
	require.Empty(t, out.SuspectedCauses)
	require.Empty(t, out.RecommendedChecks)
	require.Len(t, out.RelatedIncidents, 1)
	require.NotNil(t, out.DocsLinks)
}
