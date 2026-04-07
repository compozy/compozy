package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
)

const archiveTimestampFormat = "20060102-150405"

func archiveTaskWorkflows(ctx context.Context, cfg ArchiveConfig) (*ArchiveResult, error) {
	target, rootDir, singleWorkflow, err := resolveArchiveTarget(cfg)
	result := &ArchiveResult{
		Target:         target,
		ArchiveRoot:    model.ArchivedTasksDir(rootDir),
		SkippedReasons: make(map[string]string),
	}
	if err != nil {
		return result, err
	}

	if singleWorkflow {
		if err := archiveWorkflow(target, result); err != nil {
			return result, err
		}
		sortArchiveResult(result)
		return result, nil
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return result, fmt.Errorf("read archive target: %w", err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if !entry.IsDir() || !model.IsActiveWorkflowDirName(entry.Name()) {
			continue
		}
		if err := archiveWorkflow(filepath.Join(target, entry.Name()), result); err != nil {
			return result, err
		}
	}

	sortArchiveResult(result)
	return result, nil
}

func resolveArchiveTarget(cfg ArchiveConfig) (string, string, bool, error) {
	name := strings.TrimSpace(cfg.Name)
	if name == model.ArchivedWorkflowDirName {
		return "", "", false, fmt.Errorf("archive target cannot be %s", model.ArchivedWorkflowDirName)
	}

	resolved, err := resolveWorkflowTarget(workflowTargetOptions{
		command:       "archive",
		workspaceRoot: cfg.WorkspaceRoot,
		rootDir:       cfg.RootDir,
		name:          name,
		tasksDir:      cfg.TasksDir,
		selectorFlags: "--name or --tasks-dir",
	})
	if err != nil {
		return "", "", false, err
	}
	if pathContainsArchivedComponent(resolved.target) {
		return "", "", false, fmt.Errorf("archive target cannot be inside %s", model.ArchivedWorkflowDirName)
	}
	return resolved.target, resolved.rootDir, resolved.specificTarget, nil
}

func archiveWorkflow(tasksDir string, result *ArchiveResult) error {
	if result == nil {
		return errors.New("archive result is required")
	}

	result.WorkflowsScanned++

	reason, err := archiveSkipReason(tasksDir)
	if err != nil {
		return err
	}
	if reason != "" {
		result.Skipped++
		result.SkippedReasons[tasksDir] = reason
		return nil
	}

	if err := os.MkdirAll(result.ArchiveRoot, 0o755); err != nil {
		return fmt.Errorf("mkdir archive root: %w", err)
	}

	archivedDir := filepath.Join(result.ArchiveRoot, archivedWorkflowName(filepath.Base(tasksDir), time.Now().UTC()))
	if err := os.Rename(tasksDir, archivedDir); err != nil {
		return fmt.Errorf("archive workflow %s: %w", tasksDir, err)
	}

	result.Archived++
	result.ArchivedPaths = append(result.ArchivedPaths, archivedDir)
	return nil
}

func archiveSkipReason(tasksDir string) (string, error) {
	if _, err := os.Stat(tasks.MetaPath(tasksDir)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "missing task _meta.md", nil
		}
		return "", fmt.Errorf("stat task meta for %s: %w", tasksDir, err)
	}

	taskMeta, err := tasks.RefreshTaskMeta(tasksDir)
	if err != nil {
		return "", err
	}
	if taskMeta.Total == 0 {
		return "no task files present", nil
	}
	if taskMeta.Pending > 0 {
		return "task workflow not fully completed", nil
	}

	rounds, err := reviews.DiscoverRounds(tasksDir)
	if err != nil {
		return "", err
	}
	for _, round := range rounds {
		reviewDir := reviews.ReviewDirectory(tasksDir, round)
		if _, err := os.Stat(reviews.MetaPath(reviewDir)); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "missing review _meta.md", nil
			}
			return "", fmt.Errorf("stat review meta for %s: %w", reviewDir, err)
		}

		reviewMeta, err := reviews.RefreshRoundMeta(reviewDir)
		if err != nil {
			return "", err
		}
		if reviewMeta.Unresolved > 0 {
			return "review rounds not fully resolved", nil
		}
	}

	return "", nil
}

func archivedWorkflowName(name string, now time.Time) string {
	return fmt.Sprintf("%s-%s", now.UTC().Format(archiveTimestampFormat), name)
}

func pathContainsArchivedComponent(path string) bool {
	cleaned := filepath.Clean(path)
	for {
		if filepath.Base(cleaned) == model.ArchivedWorkflowDirName {
			return true
		}
		parent := filepath.Dir(cleaned)
		if parent == cleaned {
			return false
		}
		cleaned = parent
	}
}

func sortArchiveResult(result *ArchiveResult) {
	if result == nil {
		return
	}
	sort.Strings(result.ArchivedPaths)
}
