package plan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/compozy/looper/internal/looper/model"
	"github.com/compozy/looper/internal/looper/prompt"
	"github.com/compozy/looper/internal/looper/reviews"
)

func resolveInputs(cfg *model.RuntimeConfig) (string, string, string, error) {
	if cfg.Mode == model.ExecutionModePRDTasks {
		return resolveTaskInputs(cfg)
	}
	return resolveReviewInputs(cfg)
}

func resolveTaskInputs(cfg *model.RuntimeConfig) (string, string, string, error) {
	name := strings.TrimSpace(cfg.Name)
	tasksDir := strings.TrimSpace(cfg.TasksDir)
	if name == "" && tasksDir == "" {
		return "", "", "", missingRequiredInputsError(cfg.Mode)
	}

	var err error
	if name == "" {
		name, err = inferTaskNameFromTasksDir(tasksDir)
		if err != nil {
			return "", "", "", err
		}
	}
	if tasksDir == "" {
		tasksDir = filepath.Join("tasks", "prd-"+name)
	}

	resolvedTasksDir, err := filepath.Abs(tasksDir)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve tasks dir: %w", err)
	}
	if err := ensureDirectoryExists(resolvedTasksDir); err != nil {
		return "", "", "", err
	}

	cfg.Name = name
	cfg.TasksDir = resolvedTasksDir
	return name, tasksDir, resolvedTasksDir, nil
}

func resolveReviewInputs(cfg *model.RuntimeConfig) (string, string, string, error) {
	name := strings.TrimSpace(cfg.Name)
	reviewsDir := strings.TrimSpace(cfg.ReviewsDir)
	if name == "" && reviewsDir == "" {
		return "", "", "", missingRequiredInputsError(cfg.Mode)
	}

	if reviewsDir == "" {
		prdDir := reviews.PRDDirectory(name)
		resolvedPRDDir, err := filepath.Abs(prdDir)
		if err != nil {
			return "", "", "", fmt.Errorf("resolve prd dir: %w", err)
		}
		if err := ensureDirectoryExists(resolvedPRDDir); err != nil {
			return "", "", "", err
		}

		round := cfg.Round
		if round <= 0 {
			round, err = reviews.LatestRound(resolvedPRDDir)
			if err != nil {
				if errors.Is(err, reviews.ErrNoReviewRounds) {
					return "", "", "", fmt.Errorf("no review rounds found in %s", resolvedPRDDir)
				}
				return "", "", "", err
			}
		}
		cfg.Round = round
		reviewsDir = filepath.Join(prdDir, reviews.RoundDirName(round))
	}

	resolvedReviewsDir, err := filepath.Abs(reviewsDir)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve reviews dir: %w", err)
	}
	if err := ensureDirectoryExists(resolvedReviewsDir); err != nil {
		return "", "", "", err
	}

	if name == "" {
		name, err = inferTaskNameFromReviewsDir(resolvedReviewsDir)
		if err != nil {
			return "", "", "", err
		}
	}
	if cfg.Round <= 0 {
		round, err := inferRoundFromReviewsDir(resolvedReviewsDir)
		if err != nil {
			return "", "", "", err
		}
		cfg.Round = round
	}

	cfg.Name = name
	cfg.ReviewsDir = resolvedReviewsDir
	return name, reviewsDir, resolvedReviewsDir, nil
}

func ensureDirectoryExists(dir string) error {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("input directory not found: %s", dir)
	}
	return nil
}

func missingRequiredInputsError(mode model.ExecutionMode) error {
	if mode == model.ExecutionModePRDTasks {
		return errors.New("missing required flags: either --name or --tasks-dir must be provided")
	}
	return errors.New("missing required flags: either --name or --reviews-dir must be provided")
}

func validateAndFilterEntries(entries []model.IssueEntry, cfg *model.RuntimeConfig) ([]model.IssueEntry, error) {
	if len(entries) == 0 {
		if cfg.Mode == model.ExecutionModePRDTasks {
			fmt.Println("No task files found.")
		} else {
			fmt.Println("No review issue files found.")
		}
		return nil, ErrNoWork
	}

	if cfg.Mode == model.ExecutionModePRReview && !cfg.IncludeResolved {
		entries = filterUnresolved(entries)
		if len(entries) == 0 {
			fmt.Println("All review issues are already resolved. Nothing to do.")
			return nil, ErrNoWork
		}
	}

	return entries, nil
}

func readIssueEntries(
	resolvedInputDir string,
	mode model.ExecutionMode,
	includeCompleted bool,
) ([]model.IssueEntry, error) {
	if mode == model.ExecutionModePRDTasks {
		return readTaskEntries(resolvedInputDir, includeCompleted)
	}
	return reviews.ReadReviewEntries(resolvedInputDir)
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

func filterUnresolved(all []model.IssueEntry) []model.IssueEntry {
	out := make([]model.IssueEntry, 0, len(all))
	for _, entry := range all {
		if !prompt.IsReviewResolved(entry.Content) {
			out = append(out, entry)
		}
	}
	return out
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
		groupFile := reviews.GroupedFilePath(filepath.Dir(groupedDir), codeFile)
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
		"> This grouped file contains multiple review issues for the same source file.\n",
	)
	checklist.WriteString("> Resolve every item below before treating this group as complete.\n")
	checklist.WriteString(
		"> Looper resolves provider threads automatically after a successful batch.\n",
	)
	checklist.WriteString("> Do not run provider-specific scripts manually.\n\n")
	for _, item := range items {
		checklist.WriteString("- [ ] Resolve `")
		checklist.WriteString(item.Name)
		checklist.WriteString("` (source issue: `")
		checklist.WriteString(prompt.NormalizeForPrompt(item.AbsPath))
		checklist.WriteString("`)\n")
		checklist.WriteString("      - Triage the issue and update `## Status:` in the original issue file.\n")
		checklist.WriteString("      - Implement and verify any required code changes before marking it resolved.\n")
	}
	checklist.WriteString("- [ ] Document the outcome in this grouped file after every listed issue is resolved.\n\n")
	return checklist.String()
}

func inferTaskNameFromTasksDir(dir string) (string, error) {
	re := regexp.MustCompile(`(?:^|/)tasks/prd-([^/]+)$`)
	m := re.FindStringSubmatch(filepath.ToSlash(filepath.Clean(dir)))
	if len(m) < 2 {
		return "", errors.New("unable to infer task name from tasks dir")
	}
	return m[1], nil
}

func inferTaskNameFromReviewsDir(dir string) (string, error) {
	re := regexp.MustCompile(`(?:^|/)tasks/prd-([^/]+)/reviews-\d+$`)
	m := re.FindStringSubmatch(filepath.ToSlash(filepath.Clean(dir)))
	if len(m) < 2 {
		return "", errors.New("unable to infer task name from reviews dir")
	}
	return m[1], nil
}

func inferRoundFromReviewsDir(dir string) (int, error) {
	re := regexp.MustCompile(`reviews-(\d+)$`)
	m := re.FindStringSubmatch(filepath.ToSlash(filepath.Clean(dir)))
	if len(m) < 2 {
		return 0, errors.New("unable to infer review round from reviews dir")
	}
	round, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf("parse review round: %w", err)
	}
	return round, nil
}

func writeSummaries(reviewDir string, groups map[string][]model.IssueEntry) error {
	groupedDir := reviews.GroupedDirectory(reviewDir)
	if err := os.MkdirAll(groupedDir, 0o755); err != nil {
		return fmt.Errorf("mkdir grouped dir: %w", err)
	}
	return writeGroupedSummaries(groupedDir, groups)
}

func initPromptRoot(cfg *model.RuntimeConfig) (string, error) {
	var label string
	if cfg.Mode == model.ExecutionModePRDTasks {
		label = "tasks-" + prompt.SafeFileName(cfg.Name)
	} else {
		scope := cfg.Name
		if scope == "" {
			scope = "pr-" + cfg.PR
		}
		label = fmt.Sprintf("reviews-%s-round-%03d", prompt.SafeFileName(scope), cfg.Round)
	}

	promptRoot, err := filepath.Abs(filepath.Join(".tmp", "codex-prompts", label))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(promptRoot, 0o755); err != nil {
		return "", fmt.Errorf("mkdir prompt root: %w", err)
	}
	return promptRoot, nil
}
