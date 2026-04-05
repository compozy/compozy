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

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
)

func resolveInputs(cfg *model.RuntimeConfig) (string, string, string, error) {
	if cfg.Mode == model.ExecutionModeExec {
		return resolveExecInputs(cfg)
	}
	if cfg.Mode == model.ExecutionModePRDTasks {
		return resolveTaskInputs(cfg)
	}
	return resolveReviewInputs(cfg)
}

func resolveExecInputs(cfg *model.RuntimeConfig) (string, string, string, error) {
	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = "exec"
	}
	return cfg.Name, "", "", nil
}

func resolveExecPrompt(cfg *model.RuntimeConfig) (string, error) {
	if trimmed := strings.TrimSpace(cfg.ResolvedPromptText); trimmed != "" {
		return cfg.ResolvedPromptText, nil
	}
	if trimmed := strings.TrimSpace(cfg.PromptText); trimmed != "" {
		return cfg.PromptText, nil
	}
	if strings.TrimSpace(cfg.PromptFile) != "" {
		resolvedPath, err := filepath.Abs(cfg.PromptFile)
		if err != nil {
			return "", fmt.Errorf("resolve prompt file: %w", err)
		}
		content, err := os.ReadFile(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("read prompt file %s: %w", resolvedPath, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			return "", fmt.Errorf("prompt file %s is empty", resolvedPath)
		}
		cfg.PromptFile = resolvedPath
		cfg.ResolvedPromptText = string(content)
		return cfg.ResolvedPromptText, nil
	}
	if cfg.ReadPromptStdin {
		return "", errors.New("exec stdin prompt was not resolved before planning")
	}
	return "", errors.New("exec prompt is empty")
}

func resolveTaskInputs(cfg *model.RuntimeConfig) (string, string, string, error) {
	name := strings.TrimSpace(cfg.Name)
	tasksDir := strings.TrimSpace(cfg.TasksDir)
	if name == "" && tasksDir == "" {
		return "", "", "", missingRequiredInputsError(cfg.Mode)
	}

	var err error
	if name == "" {
		resolvedTasksDir, resolveErr := filepath.Abs(tasksDir)
		if resolveErr != nil {
			return "", "", "", fmt.Errorf("resolve tasks dir: %w", resolveErr)
		}
		name, err = inferTaskNameFromTasksDir(resolvedTasksDir, cfg.WorkspaceRoot)
		if err != nil {
			return "", "", "", err
		}
	}
	if tasksDir == "" {
		tasksDir = model.TaskDirectoryForWorkspace(cfg.WorkspaceRoot, name)
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
		prdDir := reviews.TaskDirectoryForWorkspace(cfg.WorkspaceRoot, name)
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
		name, err = inferTaskNameFromReviewsDir(resolvedReviewsDir, cfg.WorkspaceRoot)
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
			if !cfg.IncludeCompleted && strings.TrimSpace(cfg.TasksDir) != "" {
				meta, err := tasks.ReadTaskMeta(cfg.TasksDir)
				if err == nil && meta.Total > 0 && meta.Pending == 0 {
					fmt.Println("All task files are already completed. Nothing to do.")
					return nil, ErrNoWork
				}
			}
			fmt.Println("No task files found.")
		} else {
			fmt.Println("No review issue files found.")
		}
		return nil, ErrNoWork
	}

	if cfg.Mode == model.ExecutionModePRReview && !cfg.IncludeResolved {
		filtered, err := filterUnresolved(entries)
		if err != nil {
			return nil, err
		}
		entries = filtered
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
		task, err := prompt.ParseTaskFile(content)
		if err != nil {
			return nil, wrapTaskParseError(absPath, err)
		}
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

func filterUnresolved(all []model.IssueEntry) ([]model.IssueEntry, error) {
	out := make([]model.IssueEntry, 0, len(all))
	for _, entry := range all {
		resolved, err := prompt.IsReviewResolved(entry.Content)
		if err != nil {
			return nil, wrapReviewParseError(entry.AbsPath, err)
		}
		if !resolved {
			out = append(out, entry)
		}
	}
	return out, nil
}

func groupIssues(entries []model.IssueEntry) map[string][]model.IssueEntry {
	groups := make(map[string][]model.IssueEntry)
	for _, entry := range entries {
		groups[entry.CodeFile] = append(groups[entry.CodeFile], entry)
	}
	return groups
}

func inferTaskNameFromTasksDir(dir, _ string) (string, error) {
	baseDir := regexp.QuoteMeta(filepath.ToSlash(model.TasksBaseDir()))
	re := regexp.MustCompile(`(?:^|/)` + baseDir + `/([^/]+)$`)
	m := re.FindStringSubmatch(filepath.ToSlash(filepath.Clean(dir)))
	if len(m) < 2 {
		return "", fmt.Errorf(
			"unable to infer task name from tasks dir; expected path ending with %s/<name>",
			filepath.ToSlash(model.TasksBaseDir()),
		)
	}
	return m[1], nil
}

func inferTaskNameFromReviewsDir(dir, _ string) (string, error) {
	baseDir := regexp.QuoteMeta(filepath.ToSlash(model.TasksBaseDir()))
	re := regexp.MustCompile(`(?:^|/)` + baseDir + `/([^/]+)/reviews-\d+$`)
	m := re.FindStringSubmatch(filepath.ToSlash(filepath.Clean(dir)))
	if len(m) < 2 {
		return "", fmt.Errorf(
			"unable to infer task name from reviews dir; expected path ending with %s/<name>/reviews-NNN",
			filepath.ToSlash(model.TasksBaseDir()),
		)
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

func wrapTaskParseError(path string, err error) error {
	if errors.Is(err, prompt.ErrLegacyTaskMetadata) || errors.Is(err, prompt.ErrV1TaskMetadata) {
		return fmt.Errorf("legacy task artifact detected at %s; run `compozy migrate`", path)
	}
	return fmt.Errorf("parse task artifact %s: %w", path, err)
}

func wrapReviewParseError(path string, err error) error {
	if errors.Is(err, prompt.ErrLegacyReviewMetadata) {
		return fmt.Errorf("legacy review artifact detected at %s; run `compozy migrate`", path)
	}
	return fmt.Errorf("parse review artifact %s: %w", path, err)
}
