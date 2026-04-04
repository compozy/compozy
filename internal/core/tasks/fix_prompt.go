package tasks

import (
	"fmt"
	"strings"
)

// FixPrompt renders a deterministic LLM-ready remediation prompt for a validation report.
func FixPrompt(report Report, registry *TypeRegistry) string {
	if report.OK() {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(
		&b,
		"Fix the Compozy task metadata files below.\nAllowed task types: %s\n\n",
		strings.Join(registry.Values(), ", "),
	)
	b.WriteString(
		"Rewrite the YAML front matter to schema v2, remove legacy keys, preserve the body content, and keep the front matter title synced with the first H1 after stripping any `Task N:` or `Task N -` prefix.\n\n",
	)
	currentPath := ""
	for _, issue := range report.Issues {
		if issue.Path != currentPath {
			if currentPath != "" {
				b.WriteByte('\n')
			}
			currentPath = issue.Path
			b.WriteString(currentPath + "\n")
		}
		fmt.Fprintf(&b, "- %s: %s\n", issue.Field, issue.Message)
	}
	b.WriteString("\nReturn only the corrected Markdown for each file.")
	return b.String()
}
