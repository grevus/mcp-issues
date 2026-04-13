package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/tracker"
	"github.com/stretchr/testify/require"
)

type fakeReleaseLister struct {
	issues []tracker.Issue
	err    error
	gotP   tracker.ListParams
}

func (f *fakeReleaseLister) ListIssues(_ context.Context, p tracker.ListParams) ([]tracker.Issue, error) {
	f.gotP = p
	return f.issues, f.err
}

type fakeReleaseRetriever struct {
	hits    []knowledge.Hit
	err     error
	gotQ    string
	gotTopK int
}

func (f *fakeReleaseRetriever) Search(_ context.Context, _, q string, topK int) ([]knowledge.Hit, error) {
	f.gotQ = q
	f.gotTopK = topK
	return f.hits, f.err
}

func TestReleaseRiskCheck_HappyPath(t *testing.T) {
	l := &fakeReleaseLister{issues: []tracker.Issue{
		{Key: "A-1", Status: "In Progress"},
		{Key: "A-2", Status: "To Do"},
		{Key: "A-3", Status: "Blocked"},
		{Key: "A-4", Status: "Done"},
		{Key: "A-5", Status: "Closed"},
	}}
	r := &fakeReleaseRetriever{hits: []knowledge.Hit{{DocKey: "PM-1"}, {DocKey: "PM-2"}}}

	h := ReleaseRiskCheck(l, r)
	out, err := h(context.Background(), ReleaseRiskCheckInput{
		ProjectKey:       "ABC",
		FixVersion:       "1.2.3",
		ServicesInvolved: []string{"svcA", "svcB"},
	})
	require.NoError(t, err)
	require.Equal(t, "1.2.3", out.FixVersion)
	require.Len(t, out.OpenIssues, 2)
	require.Len(t, out.BlockedIssues, 1)
	require.Len(t, out.RelatedPostmortems, 2)
	require.Equal(t, "medium", out.RiskLevel)
	require.Equal(t, []string{}, out.MissingRunbooks)
	require.NotNil(t, out.MissingRunbooks)
	require.Contains(t, out.Summary, "Release 1.2.3")
	require.Contains(t, r.gotQ, "postmortem incident 1.2.3 svcA svcB")
	require.Equal(t, 5, r.gotTopK)
	require.Equal(t, "ABC", l.gotP.ProjectKey)
	require.Equal(t, "1.2.3", l.gotP.FixVersion)
	require.Equal(t, 100, l.gotP.Limit)
}

func TestReleaseRiskCheck_EmptyServices(t *testing.T) {
	l := &fakeReleaseLister{}
	r := &fakeReleaseRetriever{}
	h := ReleaseRiskCheck(l, r)
	_, err := h(context.Background(), ReleaseRiskCheckInput{ProjectKey: "P", FixVersion: "v1"})
	require.NoError(t, err)
	require.Equal(t, "postmortem incident v1", r.gotQ)
}

func TestReleaseRiskCheck_RiskLevels(t *testing.T) {
	cases := []struct {
		name              string
		open, blk, pm     int
		want              string
	}{
		{"low", 2, 0, 0, "low"},
		{"medium-blocked", 1, 1, 0, "medium"},
		{"medium-open", 10, 0, 0, "medium"},
		{"high-blocked", 0, 3, 0, "high"},
		{"high-postmortems", 0, 0, 3, "high"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, computeReleaseRisk(tc.open, tc.blk, tc.pm))
		})
	}
}

func TestReleaseRiskCheck_Validation(t *testing.T) {
	h := ReleaseRiskCheck(&fakeReleaseLister{}, &fakeReleaseRetriever{})
	_, err := h(context.Background(), ReleaseRiskCheckInput{ProjectKey: "P"})
	require.Error(t, err)
	_, err = h(context.Background(), ReleaseRiskCheckInput{FixVersion: "v1"})
	require.Error(t, err)
	_, err = h(context.Background(), ReleaseRiskCheckInput{ProjectKey: "P", FixVersion: "v1", TopK: 21})
	require.Error(t, err)
}

func TestReleaseRiskCheck_ListerError(t *testing.T) {
	l := &fakeReleaseLister{err: errors.New("boom")}
	h := ReleaseRiskCheck(l, &fakeReleaseRetriever{})
	_, err := h(context.Background(), ReleaseRiskCheckInput{ProjectKey: "P", FixVersion: "v1"})
	require.Error(t, err)
}

func TestReleaseRiskCheck_RetrieverError(t *testing.T) {
	l := &fakeReleaseLister{}
	r := &fakeReleaseRetriever{err: errors.New("rag down")}
	h := ReleaseRiskCheck(l, r)
	_, err := h(context.Background(), ReleaseRiskCheckInput{ProjectKey: "P", FixVersion: "v1"})
	require.Error(t, err)
}
