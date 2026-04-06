package jira

import "time"

type ListIssuesParams struct {
	ProjectKey string
	Status     string
	Assignee   string
	Limit      int // default 25, max 100 (применяется в client, не здесь)
}

type Issue struct {
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Assignee string `json:"assignee,omitempty"`
}

type SprintHealth struct {
	BoardID    int     `json:"board_id"`
	SprintName string  `json:"sprint_name"`
	Total      int     `json:"total"`
	Done       int     `json:"done"`
	InProgress int     `json:"in_progress"`
	Blocked    int     `json:"blocked"`
	Velocity   float64 `json:"velocity"`
}

type IssueDoc struct {
	ProjectKey    string
	Key           string
	Summary       string
	Status        string
	Assignee      string
	Description   string
	Comments      []string  // плоский список текстов
	StatusHistory []string  // "2026-01-01: To Do → In Progress"
	LinkedIssues  []string  // ключи связанных
	UpdatedAt     time.Time
}
