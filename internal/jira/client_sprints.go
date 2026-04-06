package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// sprintListResponse — приватный DTO для парсинга ответа
// GET /rest/agile/1.0/board/{boardID}/sprint?state=active.
type sprintListResponse struct {
	Values []sprintValue `json:"values"`
}

type sprintValue struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// fetchActiveSprint запрашивает активный спринт для указанного boardID.
// Возвращает sprintID и name. Используется в GetSprintHealth (Task 14) и
// будет переиспользован в Task 15 для агрегации задач.
func (c *HTTPClient) fetchActiveSprint(ctx context.Context, boardID int) (sprintID int, name string, err error) {
	path := "/rest/agile/1.0/board/" + strconv.Itoa(boardID) + "/sprint?state=active"

	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if err := checkStatus(resp, "GET", path); err != nil {
		return 0, "", err
	}

	var sl sprintListResponse
	if err := json.NewDecoder(resp.Body).Decode(&sl); err != nil {
		return 0, "", fmt.Errorf("jira: fetchActiveSprint: decode response: %w", err)
	}

	if len(sl.Values) == 0 {
		return 0, "", fmt.Errorf("jira: GetSprintHealth: no active sprint for board %d", boardID)
	}

	s := sl.Values[0]
	return s.ID, s.Name, nil
}

// GetSprintHealth возвращает SprintHealth для активного спринта boardID.
// В этой реализации (Task 14) заполняются только BoardID и SprintName.
// Агрегация задач (Total, Done, InProgress, Blocked, Velocity) добавляется в Task 15.
func (c *HTTPClient) GetSprintHealth(ctx context.Context, boardID int) (SprintHealth, error) {
	_, name, err := c.fetchActiveSprint(ctx, boardID)
	if err != nil {
		return SprintHealth{}, err
	}

	return SprintHealth{
		BoardID:    boardID,
		SprintName: name,
	}, nil
}
