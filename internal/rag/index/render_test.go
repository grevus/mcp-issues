package index

import (
	"testing"
	"time"

	"github.com/grevus/mcp-jira/internal/jira"
	"github.com/stretchr/testify/require"
)

func TestRenderDoc_Full(t *testing.T) {
	doc := jira.IssueDoc{
		ProjectKey:  "PROJ",
		Key:         "PROJ-42",
		Summary:     "Fix the auth bug",
		Status:      "In Progress",
		Assignee:    "alice",
		Description: "The authentication service fails on empty tokens.",
		Comments: []string{
			"Looks like a nil check issue.",
			"Fixed in branch feat/auth-nil.",
		},
		StatusHistory: []string{
			"2026-01-10: To Do → In Progress",
			"2026-01-12: In Progress → In Review",
		},
		LinkedIssues: []string{"PROJ-1", "PROJ-7"},
		UpdatedAt:    time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
	}

	want := `[KEY] PROJ-42
Summary: Fix the auth bug
Status: In Progress
Assignee: alice

Description:
The authentication service fails on empty tokens.

Comments:
- Looks like a nil check issue.
- Fixed in branch feat/auth-nil.

Status history:
- 2026-01-10: To Do → In Progress
- 2026-01-12: In Progress → In Review

Linked issues: PROJ-1, PROJ-7`

	require.Equal(t, want, RenderDoc(doc))
}

func TestRenderDoc_Minimal(t *testing.T) {
	doc := jira.IssueDoc{
		ProjectKey:  "ABC",
		Key:         "ABC-1",
		Summary:     "Simple task",
		Status:      "To Do",
		Assignee:    "",
		Description: "",
		// no Comments, StatusHistory, LinkedIssues
	}

	want := `[KEY] ABC-1
Summary: Simple task
Status: To Do
Assignee:

Description:
`

	require.Equal(t, want, RenderDoc(doc))
}
