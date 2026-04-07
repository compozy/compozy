package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
	"gopkg.in/yaml.v3"
)

var validTaskStatuses = []string{"pending", "in_progress", "completed", "blocked"}

var validTaskComplexities = []string{"low", "medium", "high", "critical"}

// Issue describes one validation problem found in a task file.
type Issue struct {
	Path    string `json:"path"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Report captures the outcome of validating a tasks directory.
type Report struct {
	TasksDir string  `json:"tasks_dir"`
	Scanned  int     `json:"scanned"`
	Issues   []Issue `json:"issues"`
}

// OK reports whether the validation pass found no issues.
func (r Report) OK() bool {
	return len(r.Issues) == 0
}

// Validate scans task markdown files and reports schema violations without mutating the filesystem.
func Validate(ctx context.Context, tasksDir string, registry *TypeRegistry) (Report, error) {
	if registry == nil {
		return Report{}, errors.New("task type registry is required")
	}
	if err := context.Cause(ctx); err != nil {
		return Report{}, fmt.Errorf("validate tasks: %w", err)
	}

	resolvedDir, err := filepath.Abs(strings.TrimSpace(tasksDir))
	if err != nil {
		return Report{}, fmt.Errorf("resolve tasks dir: %w", err)
	}

	report := Report{TasksDir: resolvedDir}
	entries, err := os.ReadDir(resolvedDir)
	if err != nil {
		return report, fmt.Errorf("read tasks directory: %w", err)
	}

	taskNames := collectTaskFileNames(entries)
	report.Scanned = len(taskNames)
	if len(taskNames) == 0 {
		return report, nil
	}

	existingTasks := make(map[string]struct{}, len(taskNames))
	for _, name := range taskNames {
		existingTasks[strings.TrimSuffix(name, filepath.Ext(name))] = struct{}{}
	}

	for _, name := range taskNames {
		if err := context.Cause(ctx); err != nil {
			return report, fmt.Errorf("validate tasks: %w", err)
		}

		path := filepath.Join(resolvedDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			return report, fmt.Errorf("read %s: %w", name, err)
		}

		task, body, legacyKeys, err := parseTaskForValidation(string(content))
		if err != nil {
			return report, fmt.Errorf("parse task %s: %w", name, err)
		}

		report.Issues = append(
			report.Issues,
			validateTaskFile(path, task, body, legacyKeys, registry, existingTasks)...)
	}

	return report, nil
}

func collectTaskFileNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() || ExtractTaskNumber(entry.Name()) == 0 {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.SortStableFunc(names, func(a, b string) int {
		if numA, numB := ExtractTaskNumber(a), ExtractTaskNumber(b); numA != numB {
			return numA - numB
		}
		return strings.Compare(a, b)
	})
	return names
}

func parseTaskForValidation(content string) (model.TaskEntry, string, []string, error) {
	parsedTask, parseErr := ParseTaskFile(content)

	var node yaml.Node
	body, err := frontmatter.Parse(content, &node)
	if err != nil {
		if errors.Is(parseErr, ErrLegacyTaskMetadata) {
			return model.TaskEntry{}, "", nil, ErrLegacyTaskMetadata
		}
		return model.TaskEntry{}, "", nil, err
	}

	if parseErr == nil {
		return parsedTask, body, taskLegacyKeys(&node), nil
	}
	if errors.Is(parseErr, ErrLegacyTaskMetadata) {
		return model.TaskEntry{}, "", nil, parseErr
	}

	var meta model.TaskFileMeta
	if err := node.Decode(&meta); err != nil {
		return model.TaskEntry{}, "", nil, fmt.Errorf("decode task front matter: %w", err)
	}
	return taskEntryFromMeta(content, meta), body, taskLegacyKeys(&node), nil
}

func taskEntryFromMeta(content string, meta model.TaskFileMeta) model.TaskEntry {
	return model.TaskEntry{
		Content:      content,
		Status:       strings.TrimSpace(meta.Status),
		Title:        strings.TrimSpace(meta.Title),
		TaskType:     strings.TrimSpace(meta.TaskType),
		Complexity:   strings.TrimSpace(meta.Complexity),
		Dependencies: normalizeDependencies(meta.Dependencies),
	}
}

func taskLegacyKeys(node *yaml.Node) []string {
	mapping := node
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return nil
		}
		mapping = node.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return nil
	}

	keys := make([]string, 0, 2)
	for idx := 0; idx+1 < len(mapping.Content); idx += 2 {
		keyNode := mapping.Content[idx]
		if keyNode.Kind != yaml.ScalarNode {
			continue
		}
		switch key := strings.ToLower(strings.TrimSpace(keyNode.Value)); key {
		case "domain", "scope":
			keys = append(keys, key)
		}
	}
	return keys
}

func validateTaskFile(
	path string,
	task model.TaskEntry,
	body string,
	legacyKeys []string,
	registry *TypeRegistry,
	existingTasks map[string]struct{},
) []Issue {
	issues := make([]Issue, 0, 8)
	if task.Title == "" {
		issues = append(issues, Issue{Path: path, Field: "title", Message: "title is required"})
	} else if bodyTitle := ExtractTaskBodyTitle(body); bodyTitle == "" || bodyTitle != task.Title {
		issues = append(issues, Issue{
			Path:    path,
			Field:   "title_h1_sync",
			Message: fmt.Sprintf("title %q must match the first H1 %q", task.Title, bodyTitle),
		})
	}

	if !registry.IsAllowed(task.TaskType) {
		issues = append(issues, Issue{
			Path:    path,
			Field:   "type",
			Message: fmt.Sprintf(`type %q must be one of: %s`, task.TaskType, strings.Join(registry.Values(), ", ")),
		})
	}
	if !slices.Contains(validTaskStatuses, task.Status) {
		issues = append(issues, Issue{
			Path:    path,
			Field:   "status",
			Message: fmt.Sprintf(`status %q must be one of: %s`, task.Status, strings.Join(validTaskStatuses, ", ")),
		})
	}
	if task.Complexity != "" && !slices.Contains(validTaskComplexities, task.Complexity) {
		issues = append(issues, Issue{
			Path:  path,
			Field: "complexity",
			Message: fmt.Sprintf(
				`complexity %q must be empty or one of: %s`,
				task.Complexity,
				strings.Join(validTaskComplexities, ", "),
			),
		})
	}

	if missing := missingDependencies(task.Dependencies, existingTasks); len(missing) > 0 {
		issues = append(issues, Issue{
			Path:    path,
			Field:   "dependencies",
			Message: fmt.Sprintf("dependencies reference missing tasks: %s", strings.Join(missing, ", ")),
		})
	}
	for _, key := range legacyKeys {
		issues = append(issues, Issue{
			Path:    path,
			Field:   key,
			Message: fmt.Sprintf(`legacy front matter key %q must be removed`, key),
		})
	}
	return issues
}

func missingDependencies(dependencies []string, existingTasks map[string]struct{}) []string {
	if len(dependencies) == 0 {
		return nil
	}
	missing := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		key := strings.TrimSpace(strings.TrimSuffix(dependency, filepath.Ext(dependency)))
		if key == "" {
			continue
		}
		if _, ok := existingTasks[key]; ok {
			continue
		}
		missing = append(missing, dependency)
	}
	return missing
}
