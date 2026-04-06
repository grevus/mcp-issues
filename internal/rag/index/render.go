package index

import (
	"fmt"
	"strings"

	"github.com/grevus/mcp-jira/internal/jira"
)

// RenderDoc serialises an IssueDoc into a flat text document according to
// the template defined in spec §5.5. The output is used as the content field
// stored in the vector index (one issue = one document).
func RenderDoc(d jira.IssueDoc) string {
	var b strings.Builder

	fmt.Fprintf(&b, "[KEY] %s\n", d.Key)
	fmt.Fprintf(&b, "Summary: %s\n", d.Summary)
	fmt.Fprintf(&b, "Status: %s\n", d.Status)
	if d.Assignee != "" {
		fmt.Fprintf(&b, "Assignee: %s\n", d.Assignee)
	} else {
		b.WriteString("Assignee:\n")
	}

	b.WriteString("\nDescription:\n")
	b.WriteString(d.Description)

	if len(d.Comments) > 0 {
		b.WriteString("\n\nComments:\n")
		for _, c := range d.Comments {
			fmt.Fprintf(&b, "- %s\n", c)
		}
		// trim trailing newline from last comment line
		result := b.String()
		result = strings.TrimRight(result, "\n")
		b.Reset()
		b.WriteString(result)
	}

	if len(d.StatusHistory) > 0 {
		b.WriteString("\n\nStatus history:\n")
		for _, h := range d.StatusHistory {
			fmt.Fprintf(&b, "- %s\n", h)
		}
		result := b.String()
		result = strings.TrimRight(result, "\n")
		b.Reset()
		b.WriteString(result)
	}

	if len(d.LinkedIssues) > 0 {
		fmt.Fprintf(&b, "\n\nLinked issues: %s", strings.Join(d.LinkedIssues, ", "))
	}

	return b.String()
}
