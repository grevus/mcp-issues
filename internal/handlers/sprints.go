package handlers

import (
	"context"

	"github.com/grevus/mcp-jira/internal/tracker"
)

// SprintReader — узкий интерфейс для handler SprintHealth.
type SprintReader interface {
	GetSprintHealth(ctx context.Context, boardID int) (tracker.SprintHealth, error)
}

// SprintHealthInput — параметры MCP tool get_sprint_health.
type SprintHealthInput struct {
	BoardID int `json:"board_id"`
}

// SprintHealthOutput — результат MCP tool get_sprint_health.
type SprintHealthOutput struct {
	Health tracker.SprintHealth `json:"health"`
}

// SprintHealth возвращает Handler, оборачивающий SprintReader.
func SprintHealth(r SprintReader) Handler[SprintHealthInput, SprintHealthOutput] {
	return func(ctx context.Context, in SprintHealthInput) (SprintHealthOutput, error) {
		h, err := r.GetSprintHealth(ctx, in.BoardID)
		if err != nil {
			return SprintHealthOutput{}, err
		}
		return SprintHealthOutput{Health: h}, nil
	}
}
