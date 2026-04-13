package tracker

import (
	"context"
	"time"
)

type Issue struct {
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Assignee string `json:"assignee,omitempty"`
}

type ListParams struct {
	ProjectKey  string
	Status      string
	Assignee    string
	FixVersion  string
	UpdatedFrom string
	UpdatedTo   string
	Limit       int
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

type SprintReport struct {
	Health        SprintHealth `json:"health"`
	BlockedIssues []Issue      `json:"blocked_issues"`
	ScopeAdded    []Issue      `json:"scope_added"`
	ScopeRemoved  []Issue      `json:"scope_removed"`
}

type IssueDoc struct {
	ProjectKey    string
	Key           string
	Summary       string
	Status        string
	Assignee      string
	Description   string
	Comments      []string
	StatusHistory []string
	LinkedIssues  []string
	UpdatedAt     time.Time
}

type IssueLister interface {
	ListIssues(ctx context.Context, p ListParams) ([]Issue, error)
}

type IssueFetcher interface {
	GetIssue(ctx context.Context, key string) (Issue, string, error)
}

type SprintReader interface {
	GetSprintHealth(ctx context.Context, boardID int) (SprintHealth, error)
}

type SprintReporter interface {
	GetSprintReport(ctx context.Context, boardID, sprintID int) (SprintReport, error)
}

type ScopeReader interface {
	GetSprintScopeChanges(ctx context.Context, sprintID int) (added, removed []string, err error)
}

type CommentFetcher interface {
	GetIssueComments(ctx context.Context, issueKey string) ([]string, error)
}

type DocIterator interface {
	IterateIssueDocs(ctx context.Context, projectKey string) (<-chan IssueDoc, <-chan error)
}

type Provider interface {
	IssueLister
	IssueFetcher
	SprintReader
	SprintReporter
	ScopeReader
	CommentFetcher
	DocIterator
}
