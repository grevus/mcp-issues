package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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

// sprintIssuesResponse — приватный DTO для парсинга ответа
// GET /rest/agile/1.0/sprint/{sprintID}/issue.
type sprintIssuesResponse struct {
	Issues []sprintIssue `json:"issues"`
}

type sprintIssue struct {
	Key    string            `json:"key"`
	Fields sprintIssueFields `json:"fields"`
}

type sprintIssueFields struct {
	Status          sprintIssueStatus  `json:"status"`
	StoryPoints     *float64           `json:"customfield_10016"`
}

type sprintIssueStatus struct {
	Name           string              `json:"name"`
	StatusCategory sprintStatusCategory `json:"statusCategory"`
}

type sprintStatusCategory struct {
	Key string `json:"key"`
}

// fetchSprintIssues запрашивает задачи спринта для агрегации.
func (c *HTTPClient) fetchSprintIssues(ctx context.Context, sprintID int) ([]sprintIssue, error) {
	path := "/rest/agile/1.0/sprint/" + strconv.Itoa(sprintID) + "/issue?fields=status,customfield_10016&maxResults=100"

	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkStatus(resp, "GET", path); err != nil {
		return nil, err
	}

	var sir sprintIssuesResponse
	if err := json.NewDecoder(resp.Body).Decode(&sir); err != nil {
		return nil, fmt.Errorf("jira: fetchSprintIssues: decode response: %w", err)
	}

	return sir.Issues, nil
}

// aggregateIssues вычисляет метрики из списка задач.
// Логика категоризации:
//   - Done: statusCategory.key == "done"
//   - Blocked: status.name содержит "block" (case-insensitive). Приоритет перед InProgress.
//   - InProgress: statusCategory.key == "indeterminate" И НЕ Blocked
//   - Velocity: сумма story points только для Done-задач
func aggregateIssues(issues []sprintIssue) (total, done, inProgress, blocked int, velocity float64) {
	total = len(issues)
	for _, issue := range issues {
		catKey := issue.Fields.Status.StatusCategory.Key
		statusName := strings.ToLower(issue.Fields.Status.Name)

		switch {
		case catKey == "done":
			done++
			if issue.Fields.StoryPoints != nil {
				velocity += *issue.Fields.StoryPoints
			}
		case strings.Contains(statusName, "block"):
			// Blocked — проверяем до InProgress, чтобы не двойного счёта
			blocked++
		case catKey == "indeterminate":
			inProgress++
		}
	}
	return
}

// GetSprintHealth возвращает SprintHealth для активного спринта boardID.
// Выполняет два запроса: сначала получает активный спринт, затем агрегирует его задачи.
func (c *HTTPClient) GetSprintHealth(ctx context.Context, boardID int) (SprintHealth, error) {
	sprintID, name, err := c.fetchActiveSprint(ctx, boardID)
	if err != nil {
		return SprintHealth{}, err
	}

	issues, err := c.fetchSprintIssues(ctx, sprintID)
	if err != nil {
		return SprintHealth{}, err
	}

	total, done, inProgress, blockedCount, velocity := aggregateIssues(issues)

	return SprintHealth{
		BoardID:    boardID,
		SprintName: name,
		Total:      total,
		Done:       done,
		InProgress: inProgress,
		Blocked:    blockedCount,
		Velocity:   velocity,
	}, nil
}
