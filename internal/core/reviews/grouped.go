package reviews

import (
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
)

func WriteGroupedSummaries(reviewDir string, groups map[string][]model.IssueEntry) error {
	groupedDir := GroupedDirectory(reviewDir)
	if err := os.MkdirAll(groupedDir, 0o755); err != nil {
		return fmt.Errorf("mkdir grouped dir: %w", err)
	}

	for codeFile, items := range groups {
		groupFile := GroupedFilePath(reviewDir, codeFile)
		title := codeFile
		if strings.HasPrefix(codeFile, "__unknown__") {
			title = "(unknown file)"
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# Issue Group for %s\n\n", title)
		sb.WriteString(buildGroupedResolutionChecklist(items))
		sb.WriteString("## Included Issues\n\n")
		for _, item := range items {
			sb.WriteString("- ")
			sb.WriteString(item.Name)
			sb.WriteString("\n")
		}
		for _, item := range items {
			sb.WriteString("\n---\n\n## ")
			sb.WriteString(item.Name)
			sb.WriteString("\n\n")
			sb.WriteString(item.Content)
			if !strings.HasSuffix(item.Content, "\n") {
				sb.WriteString("\n")
			}
		}
		if err := os.WriteFile(groupFile, []byte(sb.String()), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func RegenerateGroupedSummaries(reviewDir string) error {
	entries, err := ReadReviewEntries(reviewDir)
	if err != nil {
		return err
	}

	groups := make(map[string][]model.IssueEntry)
	for _, entry := range entries {
		groups[entry.CodeFile] = append(groups[entry.CodeFile], entry)
	}
	return WriteGroupedSummaries(reviewDir, groups)
}

func buildGroupedResolutionChecklist(items []model.IssueEntry) string {
	var checklist strings.Builder
	checklist.WriteString("## Resolution Checklist\n\n")
	checklist.WriteString(
		"> This grouped file contains multiple review issues for the same source file.\n",
	)
	checklist.WriteString("> Resolve every item below before treating this group as complete.\n")
	checklist.WriteString(
		"> Compozy resolves provider threads automatically after a successful batch.\n",
	)
	checklist.WriteString("> Do not run provider-specific scripts manually.\n\n")
	for _, item := range items {
		checklist.WriteString("- [ ] Resolve `")
		checklist.WriteString(item.Name)
		checklist.WriteString("` (source issue: `")
		checklist.WriteString(prompt.NormalizeForPrompt(item.AbsPath))
		checklist.WriteString("`)\n")
		checklist.WriteString("      - Triage the issue and update `status` in the issue front matter.\n")
		checklist.WriteString("      - Implement and verify any required code changes before marking it resolved.\n")
	}
	checklist.WriteString("- [ ] Document the outcome in this grouped file after every listed issue is resolved.\n\n")
	return checklist.String()
}
