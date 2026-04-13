package handlers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/stretchr/testify/require"
)

type fakeKnowledgeRetriever struct {
	gotProjectKey string
	gotQuery      string
	gotTopK       int
	hits          []knowledge.Hit
	err           error
}

func (f *fakeKnowledgeRetriever) Search(_ context.Context, projectKey, query string, topK int) ([]knowledge.Hit, error) {
	f.gotProjectKey = projectKey
	f.gotQuery = query
	f.gotTopK = topK
	return f.hits, f.err
}

func TestSearchKnowledge_HappyPath(t *testing.T) {
	fake := &fakeKnowledgeRetriever{
		hits: []knowledge.Hit{
			{DocKey: "ABC-10", Title: "Auth refactor", Status: "Done", Score: 0.95, Excerpt: "Refactor auth module"},
			{DocKey: "ABC-11", Title: "Auth middleware", Status: "In Progress", Score: 0.87, Excerpt: "Add middleware"},
		},
	}

	h := handlers.SearchKnowledge(fake)
	out, err := h(context.Background(), handlers.SearchKnowledgeInput{
		ProjectKey: "ABC",
		Query:      "auth refactor",
		TopK:       5,
	})

	require.NoError(t, err)
	require.Equal(t, "ABC", fake.gotProjectKey)
	require.Equal(t, "auth refactor", fake.gotQuery)
	require.Equal(t, 5, fake.gotTopK)
	require.Len(t, out.Hits, 2)
	require.Equal(t, "ABC-10", out.Hits[0].DocKey)
	require.Equal(t, "ABC-11", out.Hits[1].DocKey)
}

func TestSearchKnowledge_PropagatesError(t *testing.T) {
	fake := &fakeKnowledgeRetriever{
		err: errors.New("retriever error"),
	}

	h := handlers.SearchKnowledge(fake)
	_, err := h(context.Background(), handlers.SearchKnowledgeInput{
		ProjectKey: "ABC",
		Query:      "auth",
		TopK:       5,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "retriever error")
}

func TestSearchKnowledge_TopKDefault(t *testing.T) {
	fake := &fakeKnowledgeRetriever{}

	h := handlers.SearchKnowledge(fake)
	_, err := h(context.Background(), handlers.SearchKnowledgeInput{
		ProjectKey: "ABC",
		Query:      "auth",
		TopK:       0,
	})

	require.NoError(t, err)
	require.Equal(t, 5, fake.gotTopK)
}

func TestSearchKnowledge_TopKNegativeDefault(t *testing.T) {
	fake := &fakeKnowledgeRetriever{}

	h := handlers.SearchKnowledge(fake)
	_, err := h(context.Background(), handlers.SearchKnowledgeInput{
		ProjectKey: "ABC",
		Query:      "auth",
		TopK:       -3,
	})

	require.NoError(t, err)
	require.Equal(t, 5, fake.gotTopK)
}

func TestSearchKnowledge_TopKValid(t *testing.T) {
	fake := &fakeKnowledgeRetriever{}

	h := handlers.SearchKnowledge(fake)
	_, err := h(context.Background(), handlers.SearchKnowledgeInput{
		ProjectKey: "ABC",
		Query:      "auth",
		TopK:       15,
	})

	require.NoError(t, err)
	require.Equal(t, 15, fake.gotTopK)
}

func TestSearchKnowledge_TopKExceedsMax(t *testing.T) {
	fake := &fakeKnowledgeRetriever{}

	h := handlers.SearchKnowledge(fake)
	_, err := h(context.Background(), handlers.SearchKnowledgeInput{
		ProjectKey: "ABC",
		Query:      "auth",
		TopK:       21,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "top_k")
	require.Contains(t, err.Error(), "21")
	// Search не должен быть вызван — gotTopK остаётся нулём (zero value)
	require.Equal(t, 0, fake.gotTopK, "Search не должен вызываться при TopK > 20")
}
