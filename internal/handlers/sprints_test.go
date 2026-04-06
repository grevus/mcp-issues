package handlers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/jira"
	"github.com/stretchr/testify/require"
)

type fakeSprintReader struct {
	gotBoardID int
	health     jira.SprintHealth
	err        error
}

func (f *fakeSprintReader) GetSprintHealth(_ context.Context, boardID int) (jira.SprintHealth, error) {
	f.gotBoardID = boardID
	return f.health, f.err
}

func TestSprintHealth_HappyPath(t *testing.T) {
	fake := &fakeSprintReader{
		health: jira.SprintHealth{
			BoardID:    42,
			SprintName: "Sprint 1",
			Total:      10,
			Done:       7,
			InProgress: 2,
			Blocked:    1,
			Velocity:   8.5,
		},
	}

	h := handlers.SprintHealth(fake)
	out, err := h(context.Background(), handlers.SprintHealthInput{BoardID: 42})

	require.NoError(t, err)
	require.Equal(t, 42, fake.gotBoardID)
	require.Equal(t, "Sprint 1", out.Health.SprintName)
	require.Equal(t, 10, out.Health.Total)
	require.Equal(t, 7, out.Health.Done)
}

func TestSprintHealth_PropagatesError(t *testing.T) {
	fake := &fakeSprintReader{
		err: errors.New("no active sprint"),
	}

	h := handlers.SprintHealth(fake)
	_, err := h(context.Background(), handlers.SprintHealthInput{BoardID: 42})

	require.Error(t, err)
	require.ErrorContains(t, err, "no active sprint")
}
