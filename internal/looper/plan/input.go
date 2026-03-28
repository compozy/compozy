package plan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/compozy/looper/internal/looper/model"
	"github.com/compozy/looper/internal/looper/prompt"
)

func resolveInputs(cfg *model.RuntimeConfig) (string, string, string, error) {
	prValue := cfg.PR
	inputDir := cfg.IssuesDir
	if prValue == "" && inputDir == "" {
		return "", "", "", errors.New("missing required flags: either --pr or --issues-dir must be provided")
	}

	var err error
	if prValue == "" && inputDir != "" {
		prValue, err = inferPrFromIssuesDir(inputDir)
		if err != nil {
			return "", "", "", err
		}
	}

	if inputDir == "" {
		if cfg.Mode == model.ExecutionModePRDTasks {
			inputDir = fmt.Sprintf("tasks/prd-%s", prValue)
		} else {
			inputDir = fmt.Sprintf("ai-docs/reviews-pr-%s/issues", prValue)
		}
	}

	resolvedInputDir, err := filepath.Abs(inputDir)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve issues dir: %w", err)
	}
	if st, statErr := os.Stat(resolvedInputDir); statErr != nil || !st.IsDir() {
		return "", "", "", fmt.Errorf("issues directory not found: %s", resolvedInputDir)
	}
	return prValue, inputDir, resolvedInputDir, nil
}

func validateAndFilterEntries(entries []model.IssueEntry, mode model.ExecutionMode) ([]model.IssueEntry, error) {
	if len(entries) == 0 {
		if mode == model.ExecutionModePRDTasks {
			fmt.Println("No task files found.")
		} else {
			fmt.Println("No issue files found.")
		}
		return nil, ErrNoWork
	}
	if mode == model.ExecutionModePRReview {
		entries = filterUnresolved(entries)
		if len(entries) == 0 {
			fmt.Println("All issues are already resolved. Nothing to do.")
			return nil, ErrNoWork
		}
	}
	return entries, nil
}

func readIssueEntries(
	resolvedIssuesDir string,
	mode model.ExecutionMode,
	includeCompleted bool,
) ([]model.IssueEntry, error) {
	if mode == model.ExecutionModePRDTasks {
		return readTaskEntries(resolvedIssuesDir, includeCompleted)
	}
	return readCodeRabbitIssues(resolvedIssuesDir)
}

func readTaskEntries(tasksDir string, includeCompleted bool) ([]model.IssueEntry, error) {
	entries := []model.IssueEntry{}
	files, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(files))
	for _, f := range files {
		if !f.Type().IsRegular() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		if prompt.ExtractTaskNumber(f.Name()) == 0 {
			continue
		}
		names = append(names, f.Name())
	}

	sort.SliceStable(names, func(i, j int) bool {
		return prompt.ExtractTaskNumber(names[i]) < prompt.ExtractTaskNumber(names[j])
	})

	for _, name := range names {
		absPath := filepath.Join(tasksDir, name)
		body, err := os.ReadFile(absPath)
		if err != nil {
			return nil, err
		}

		content := string(body)
		task := prompt.ParseTaskFile(content)
		if !includeCompleted && prompt.IsTaskCompleted(task) {
			continue
		}

		entries = append(entries, model.IssueEntry{
			Name:     name,
			AbsPath:  absPath,
			Content:  content,
			CodeFile: strings.TrimSuffix(name, ".md"),
		})
	}
	return entries, nil
}

func readCodeRabbitIssues(resolvedIssuesDir string) ([]model.IssueEntry, error) {
	entries := []model.IssueEntry{}
	files, err := os.ReadDir(resolvedIssuesDir)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(files))
	for _, f := range files {
		if !f.Type().IsRegular() {
			continue
		}
		if f.Name() == "_summary.md" {
			continue
		}
		if strings.HasSuffix(f.Name(), ".md") {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		absPath := filepath.Join(resolvedIssuesDir, name)
		body, err := os.ReadFile(absPath)
		if err != nil {
			return nil, err
		}

		content := string(body)
		codeFile := extractCodeFileFromIssue(content)
		if codeFile == "" {
			codeFile = "__unknown__:" + name
		}
		entries = append(entries, model.IssueEntry{
			Name:     name,
			AbsPath:  absPath,
			Content:  content,
			CodeFile: codeFile,
		})
	}
	return entries, nil
}

func filterUnresolved(all []model.IssueEntry) []model.IssueEntry {
	out := make([]model.IssueEntry, 0, len(all))
	for _, entry := range all {
		if !isIssueResolved(entry.Content) {
			out = append(out, entry)
		}
	}
	return out
}

var (
	reResolvedStatus = regexp.MustCompile(`(?mi)^\s*(status|state)\s*:\s*resolved\b`)
	reResolvedTask   = regexp.MustCompile(`(?mi)^\s*-\s*\[(x|X)\]\s*resolved\b`)
)

func isIssueResolved(content string) bool {
	if strings.Contains(strings.ToUpper(content), "RESOLVED ✓") {
		return true
	}
	if reResolvedStatus.FindStringIndex(content) != nil {
		return true
	}
	if reResolvedTask.FindStringIndex(content) != nil {
		return true
	}
	return false
}

func groupIssues(entries []model.IssueEntry) map[string][]model.IssueEntry {
	groups := make(map[string][]model.IssueEntry)
	for _, entry := range entries {
		groups[entry.CodeFile] = append(groups[entry.CodeFile], entry)
	}
	return groups
}

func writeGroupedSummaries(groupedDir string, groups map[string][]model.IssueEntry) error {
	for codeFile, items := range groups {
		safeName := prompt.SafeFileName(func() string {
			if strings.HasPrefix(codeFile, "__unknown__") {
				return model.UnknownFileName
			}
			return codeFile
		}())

		groupFile := filepath.Join(groupedDir, fmt.Sprintf("%s.md", safeName))
		header := fmt.Sprintf("# Issue Group for %s\n\n", func() string {
			if strings.HasPrefix(codeFile, "__unknown__") {
				return "(unknown file)"
			}
			return codeFile
		}())

		var sb strings.Builder
		sb.WriteString(header)
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
		}
		sb.WriteString("\n")
		if err := os.WriteFile(groupFile, []byte(sb.String()), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func buildGroupedResolutionChecklist(items []model.IssueEntry) string {
	var checklist strings.Builder
	checklist.WriteString("## Resolution Checklist\n\n")
	checklist.WriteString(
		"> ⚠️ This grouped issue contains multiple unresolved review tasks for the same source file.\n",
	)
	checklist.WriteString("> Resolve **every** task below before treating this file as complete.\n")
	checklist.WriteString(
		"> After resolving a task, update the original issue file with `RESOLVED ✓` and run any provided gh command.\n\n",
	)
	for _, item := range items {
		checklist.WriteString("- [ ] Resolve `")
		checklist.WriteString(item.Name)
		checklist.WriteString("` (source issue: `")
		checklist.WriteString(prompt.NormalizeForPrompt(item.AbsPath))
		checklist.WriteString("`)\n")
		checklist.WriteString("      - Apply the requested code changes and update the issue status to `RESOLVED ✓`.\n")
		checklist.WriteString("      - Run the review thread command if a Thread ID is provided.\n")
	}
	checklist.WriteString("- [ ] Document the fixes in this grouped file and tick every checklist item above.\n\n")
	return checklist.String()
}

func inferPrFromIssuesDir(dir string) (string, error) {
	re := regexp.MustCompile(`reviews-pr-(\d+)`)
	m := re.FindStringSubmatch(dir)
	if len(m) < 2 {
		return "", errors.New("unable to infer PR number from issues dir")
	}
	return m[1], nil
}

func extractCodeFileFromIssue(content string) string {
	re := regexp.MustCompile(`\*\*File:\*\*\s*` + "`" + `([^` + "`" + `]+)` + "`")
	m := re.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	raw := strings.TrimSpace(m[1])
	if idx := strings.LastIndex(raw, ":"); idx != -1 {
		tail := raw[idx+1:]
		if tail != "" && isAllDigits(tail) {
			raw = strings.TrimSpace(raw[:idx])
		}
	}
	return raw
}

func isAllDigits(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func writeSummaries(resolvedIssuesDir string, groups map[string][]model.IssueEntry) error {
	groupedDir := filepath.Join(resolvedIssuesDir, "grouped")
	if err := os.MkdirAll(groupedDir, 0o755); err != nil {
		return fmt.Errorf("mkdir grouped dir: %w", err)
	}
	return writeGroupedSummaries(groupedDir, groups)
}

func initPromptRoot(pr string) (string, error) {
	promptRoot, err := filepath.Abs(filepath.Join(".tmp", "codex-prompts", fmt.Sprintf("pr-%s", pr)))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(promptRoot, 0o755); err != nil {
		return "", fmt.Errorf("mkdir prompt root: %w", err)
	}
	return promptRoot, nil
}
