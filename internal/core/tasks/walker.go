package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

func ReadTaskEntries(tasksDir string, includeCompleted bool) ([]model.IssueEntry, error) {
	entries := make([]model.IssueEntry, 0)
	if err := walkTaskFiles(tasksDir, func(entry model.IssueEntry, task model.TaskEntry) error {
		if !includeCompleted && IsTaskCompleted(task) {
			return nil
		}
		entries = append(entries, entry)
		return nil
	}); err != nil {
		return nil, err
	}
	return entries, nil
}

func walkTaskFiles(tasksDir string, visit func(model.IssueEntry, model.TaskEntry) error) error {
	names, err := taskFileNames(tasksDir)
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

func taskFileNames(tasksDir string) ([]string, error) {
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

	sort.SliceStable(names, func(i, j int) bool {
		return ExtractTaskNumber(names[i]) < ExtractTaskNumber(names[j])
	})
	return names, nil
}

func readTaskEntry(tasksDir, name string) (model.IssueEntry, model.TaskEntry, error) {
	absPath := filepath.Join(tasksDir, name)
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
