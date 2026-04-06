package jira

import (
	"fmt"
	"regexp"
	"strings"
)

var projectKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// quoteJQL escapes s for safe substitution into a JQL expression and wraps
// the result in double quotes. Backslashes are replaced before double quotes
// to avoid double-escaping.
func quoteJQL(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// validateProjectKey проверяет, что s соответствует whitelist-паттерну проектного
// ключа Jira: первый символ — заглавная буква, далее заглавные буквы, цифры или
// подчёркивание. Пустая строка и любое несоответствие возвращают ошибку.
func validateProjectKey(s string) error {
	if !projectKeyRe.MatchString(s) {
		return fmt.Errorf("jira: invalid project key %q", s)
	}
	return nil
}
