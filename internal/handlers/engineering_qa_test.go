package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/stretchr/testify/require"
)

type fakeQARetriever struct {
	hits       []knowledge.Hit
	err        error
	gotProject string
	gotQuery   string
	gotTopK    int
}

func (f *fakeQARetriever) Search(_ context.Context, projectKey, query string, topK int) ([]knowledge.Hit, error) {
	f.gotProject = projectKey
	f.gotQuery = query
	f.gotTopK = topK
	if f.err != nil {
		return nil, f.err
	}
	return f.hits, nil
}

func TestEngineeringQA_HappyPath(t *testing.T) {
	f := &fakeQARetriever{hits: []knowledge.Hit{{DocKey: "ABC-1", Title: "foo", Score: 0.9}}}
	h := EngineeringQA(f)

	out, err := h(context.Background(), EngineeringQAInput{
		Question:    "How do we rotate DB creds?",
		ProjectKey:  "ABC",
		ContextHint: "postgres vault",
		TopK:        7,
	})
	require.NoError(t, err)
	require.Equal(t, "ABC", f.gotProject)
	require.Equal(t, 7, f.gotTopK)
	require.True(t, strings.Contains(f.gotQuery, "rotate DB creds"))
	require.True(t, strings.Contains(f.gotQuery, "postgres vault"))
	require.Len(t, out.Citations, 1)
	require.Equal(t, "ABC-1", out.Citations[0].DocKey)
}

func TestEngineeringQA_Validation(t *testing.T) {
	h := EngineeringQA(&fakeQARetriever{})

	_, err := h(context.Background(), EngineeringQAInput{Question: "  ", ProjectKey: "ABC"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "question is required")

	_, err = h(context.Background(), EngineeringQAInput{Question: "q", ProjectKey: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "project_key is required")

	_, err = h(context.Background(), EngineeringQAInput{Question: "q", ProjectKey: "ABC", TopK: 21})
	require.Error(t, err)
	require.Contains(t, err.Error(), "top_k must be <= 20")
}

func TestEngineeringQA_DefaultTopK(t *testing.T) {
	f := &fakeQARetriever{hits: []knowledge.Hit{}}
	h := EngineeringQA(f)

	_, err := h(context.Background(), EngineeringQAInput{Question: "q", ProjectKey: "ABC"})
	require.NoError(t, err)
	require.Equal(t, 5, f.gotTopK)
}

func TestEngineeringQA_RetrieverError(t *testing.T) {
	f := &fakeQARetriever{err: errors.New("boom")}
	h := EngineeringQA(f)

	_, err := h(context.Background(), EngineeringQAInput{Question: "q", ProjectKey: "ABC"})
	require.Error(t, err)
}
