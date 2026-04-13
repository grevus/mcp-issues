package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/grevus/mcp-jira/internal/tracker"
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
	Summary     string            `json:"summary"`
	Status      sprintIssueStatus `json:"status"`
	StoryPoints *float64          `json:"customfield_10016"`
	Assignee    *issueAssignee    `json:"assignee"`
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
	path := "/rest/agile/1.0/sprint/" + strconv.Itoa(sprintID) + "/issue?fields=summary,status,assignee,customfield_10016&maxResults=100"

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
func (c *HTTPClient) GetSprintHealth(ctx context.Context, boardID int) (tracker.SprintHealth, error) {
	sprintID, name, err := c.fetchActiveSprint(ctx, boardID)
	if err != nil {
		return tracker.SprintHealth{}, err
	}

	issues, err := c.fetchSprintIssues(ctx, sprintID)
	if err != nil {
		return tracker.SprintHealth{}, err
	}

	total, done, inProgress, blockedCount, velocity := aggregateIssues(issues)

	return tracker.SprintHealth{
		BoardID:    boardID,
		SprintName: name,
		Total:      total,
		Done:       done,
		InProgress: inProgress,
		Blocked:    blockedCount,
		Velocity:   velocity,
	}, nil
}

// issueFromSprint конвертирует внутренний sprintIssue в публичный tracker.Issue.
func issueFromSprint(si sprintIssue) tracker.Issue {
	assignee := ""
	if si.Fields.Assignee != nil {
		assignee = si.Fields.Assignee.DisplayName
	}
	return tracker.Issue{
		Key:      si.Key,
		Summary:  si.Fields.Summary,
		Status:   si.Fields.Status.Name,
		Assignee: assignee,
	}
}

// GetSprintReport возвращает расширенный health-отчёт: агрегаты + список blocked задач.
// Если sprintID <= 0, используется активный спринт доски.
// ScopeAdded/ScopeRemoved пока не заполняются (требуется анализ changelog — phase 2).
func (c *HTTPClient) GetSprintReport(ctx context.Context, boardID, sprintID int) (tracker.SprintReport, error) {
	var (
		name string
		err  error
	)
	if sprintID <= 0 {
		sprintID, name, err = c.fetchActiveSprint(ctx, boardID)
		if err != nil {
			return tracker.SprintReport{}, err
		}
	}

	issues, err := c.fetchSprintIssues(ctx, sprintID)
	if err != nil {
		return tracker.SprintReport{}, err
	}

	total, done, inProgress, blockedCount, velocity := aggregateIssues(issues)

	blocked := make([]tracker.Issue, 0, blockedCount)
	for _, si := range issues {
		if strings.Contains(strings.ToLower(si.Fields.Status.Name), "block") {
			blocked = append(blocked, issueFromSprint(si))
		}
	}

	return tracker.SprintReport{
		Health: tracker.SprintHealth{
			BoardID:    boardID,
			SprintName: name,
			Total:      total,
			Done:       done,
			InProgress: inProgress,
			Blocked:    blockedCount,
			Velocity:   velocity,
		},
		BlockedIssues: blocked,
		ScopeAdded:    []tracker.Issue{},
		ScopeRemoved:  []tracker.Issue{},
	}, nil
}

// fetchSprintName возвращает имя спринта по его ID через Agile API.
func (c *HTTPClient) fetchSprintName(ctx context.Context, sprintID int) (string, error) {
	path := "/rest/agile/1.0/sprint/" + strconv.Itoa(sprintID)
	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, "GET", path); err != nil {
		return "", err
	}
	var sv sprintValue
	if err := json.NewDecoder(resp.Body).Decode(&sv); err != nil {
		return "", fmt.Errorf("jira: fetchSprintName: decode: %w", err)
	}
	return sv.Name, nil
}

// GetSprintScopeChanges возвращает списки issue-ключей, добавленных в спринт и
// удалённых из спринта, по истории поля Sprint (expand=changelog).
// Алгоритм:
//   - JQL `sprint = X OR sprint was X` возвращает все задачи, когда-либо
//     принадлежавшие спринту;
//   - для каждой задачи итерируем changelog.histories и ищем изменения поля
//     Sprint, где имя целевого спринта появляется в toString (added) или
//     исчезает из fromString в toString (removed);
//   - если задача была добавлена и затем удалена в рамках одной истории, она
//     попадает и в added, и в removed — это корректно отражает движение scope.
//
// Возвращает отсортированные дедуплицированные списки ключей.
func (c *HTTPClient) GetSprintScopeChanges(ctx context.Context, sprintID int) (added, removed []string, err error) {
	if sprintID <= 0 {
		return nil, nil, fmt.Errorf("jira: GetSprintScopeChanges: sprintID must be > 0")
	}

	sprintName, err := c.fetchSprintName(ctx, sprintID)
	if err != nil {
		return nil, nil, fmt.Errorf("jira: GetSprintScopeChanges: %w", err)
	}

	q := url.Values{}
	q.Set("jql", fmt.Sprintf("sprint = %d OR sprint was %d", sprintID, sprintID))
	q.Set("fields", "summary")
	q.Set("expand", "changelog")
	q.Set("maxResults", "100")

	path := "/rest/api/3/search/jql?" + q.Encode()
	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("jira: GetSprintScopeChanges: %w", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, "GET", path); err != nil {
		return nil, nil, err
	}

	var sr docsSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, nil, fmt.Errorf("jira: GetSprintScopeChanges: decode: %w", err)
	}

	addedSet := map[string]struct{}{}
	removedSet := map[string]struct{}{}

	for _, ir := range sr.Issues {
		if ir.Changelog == nil {
			continue
		}
		for _, h := range ir.Changelog.Histories {
			for _, item := range h.Items {
				if !strings.EqualFold(item.Field, "Sprint") {
					continue
				}
				inFrom := sprintNameInList(item.FromString, sprintName)
				inTo := sprintNameInList(item.ToString, sprintName)
				switch {
				case !inFrom && inTo:
					addedSet[ir.Key] = struct{}{}
				case inFrom && !inTo:
					removedSet[ir.Key] = struct{}{}
				}
			}
		}
	}

	added = keysSorted(addedSet)
	removed = keysSorted(removedSet)
	return added, removed, nil
}

// sprintNameInList проверяет, содержит ли список имён спринтов (через запятую)
// указанное имя. Сравнение case-insensitive и trim-устойчивое.
func sprintNameInList(list, name string) bool {
	if list == "" || name == "" {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(name))
	for _, part := range strings.Split(list, ",") {
		if strings.ToLower(strings.TrimSpace(part)) == target {
			return true
		}
	}
	return false
}

func keysSorted(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
