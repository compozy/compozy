package tasks

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

func ReadTaskEntries(tasksDir string, includeCompleted bool) ([]model.IssueEntry, error) {
	return readTaskEntriesWithMode(tasksDir, includeCompleted, false)
}

func ReadTaskEntriesRecursive(tasksDir string, includeCompleted bool) ([]model.IssueEntry, error) {
	return readTaskEntriesWithMode(tasksDir, includeCompleted, true)
}

func readTaskEntriesWithMode(tasksDir string, includeCompleted, recursive bool) ([]model.IssueEntry, error) {
	entries := make([]model.IssueEntry, 0)
	if err := walkTaskFiles(tasksDir, recursive, func(entry model.IssueEntry, task model.TaskEntry) error {
		if !includeCompleted && IsTaskCompleted(task) {
			return nil
		}
		entries = append(entries, entry)
		return nil
	}); err != nil {
		return nil, err
	}
	slog.Debug(
		"recursive task discovery resolved entries",
		"count", len(entries),
		"recursive", recursive,
	)
	return entries, nil
}

func walkTaskFiles(
	tasksDir string,
	recursive bool,
	visit func(model.IssueEntry, model.TaskEntry) error,
) error {
	names, err := taskFileNames(tasksDir, recursive)
	if err != nil {
		return err
	}

	for _, name := range names {
		entry, task, err := readTaskEntry(tasksDir, name)
		if err != nil {
			return err
		}
		if err := visit(entry, task); err != nil {
			return err
		}
	}
	return nil
}

func taskFileNames(tasksDir string, recursive bool) ([]string, error) {
	if recursive {
		return taskFileNamesRecursive(tasksDir)
	}
	return taskFileNamesFlat(tasksDir)
}

func taskFileNamesFlat(tasksDir string) ([]string, error) {
	files, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("read tasks directory: %w", err)
	}

	names := make([]string, 0, len(files))
	for _, file := range files {
		if !file.Type().IsRegular() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}
		if ExtractTaskNumber(file.Name()) == 0 {
			continue
		}
		names = append(names, file.Name())
	}

	sortTaskNames(names)
	return names, nil
}

func taskFileNamesRecursive(tasksDir string) ([]string, error) {
	names := make([]string, 0)
	walkErr := filepath.WalkDir(tasksDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() && path != tasksDir {
				slog.Debug(
					"recursive task discovery skipped directory",
					"dir", relForLog(tasksDir, path),
					"reason", "walk-error",
					"error", err,
				)
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() {
			if path == tasksDir {
				return nil
			}
			if shouldSkipDir(d.Name()) {
				slog.Debug(
					"recursive task discovery skipped directory",
					"dir", relForLog(tasksDir, path),
					"reason", "skip-list",
				)
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		if ExtractTaskNumber(d.Name()) == 0 {
			return nil
		}
		rel, relErr := filepath.Rel(tasksDir, path)
		if relErr != nil {
			return fmt.Errorf("relpath %s: %w", path, relErr)
		}
		names = append(names, filepath.ToSlash(rel))
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk tasks directory: %w", walkErr)
	}
	sortTaskNames(names)
	return names, nil
}

func sortTaskNames(names []string) {
	sort.SliceStable(names, func(i, j int) bool {
		di, dj := dirKey(names[i]), dirKey(names[j])
		if di != dj {
			return di < dj
		}
		ni := ExtractTaskNumber(filepath.Base(names[i]))
		nj := ExtractTaskNumber(filepath.Base(names[j]))
		if ni != nj {
			return ni < nj
		}
		return names[i] < names[j]
	})
}

func dirKey(rel string) string {
	dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel)))
	if dir == "." {
		return ""
	}
	return dir
}

func shouldSkipDir(name string) bool {
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	if strings.HasPrefix(name, "_") {
		return true
	}
	if strings.HasPrefix(name, "reviews-") {
		return true
	}
	switch name {
	case "adrs", "memory":
		return true
	}
	return false
}

func relForLog(tasksDir, path string) string {
	rel, err := filepath.Rel(tasksDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func readTaskEntry(tasksDir, name string) (model.IssueEntry, model.TaskEntry, error) {
	absPath := filepath.Join(tasksDir, filepath.FromSlash(name))
	body, err := os.ReadFile(absPath)
	if err != nil {
		return model.IssueEntry{}, model.TaskEntry{}, fmt.Errorf("read %s: %w", name, err)
	}

	content := string(body)
	task, err := ParseTaskFile(content)
	if err != nil {
		return model.IssueEntry{}, model.TaskEntry{}, WrapParseError(absPath, err)
	}

	entry := model.IssueEntry{
		Name:     name,
		AbsPath:  absPath,
		Content:  content,
		CodeFile: strings.TrimSuffix(name, filepath.Ext(name)),
	}
	return entry, task, nil
}
