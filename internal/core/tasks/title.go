package tasks

import (
	"regexp"
	"strings"
)

var taskTitlePrefixPattern = regexp.MustCompile(`^Task\s+\d+\s*(?::|-)\s*`)

// ExtractTaskBodyTitle returns the first H1 title from the body with any
// leading "Task N:" or "Task N -" prefix removed.
func ExtractTaskBodyTitle(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "# ") {
			continue
		}
		return strings.TrimSpace(
			taskTitlePrefixPattern.ReplaceAllString(strings.TrimSpace(strings.TrimPrefix(trimmed, "# ")), ""),
		)
	}
	return ""
}
