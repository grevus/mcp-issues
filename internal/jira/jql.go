package jira

import "strings"

// quoteJQL escapes s for safe substitution into a JQL expression and wraps
// the result in double quotes. Backslashes are replaced before double quotes
// to avoid double-escaping.
func quoteJQL(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
