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
	specificTargets := 0
	if strings.TrimSpace(cfg.Name) != "" {
		specificTargets++
	}
	if strings.TrimSpace(cfg.TasksDir) != "" {
		specificTargets++
	}
	if specificTargets > 1 {
		return "", "", false, errors.New("archive accepts only one of --name or --tasks-dir")
	}

	rootDir := strings.TrimSpace(cfg.RootDir)
	if rootDir == "" {
		rootDir = model.TasksBaseDirForWorkspace(cfg.WorkspaceRoot)
	}

	target := rootDir
	singleWorkflow := false
	switch {
	case strings.TrimSpace(cfg.TasksDir) != "":
		target = strings.TrimSpace(cfg.TasksDir)
		singleWorkflow = true
	case strings.TrimSpace(cfg.Name) != "":
		name := strings.TrimSpace(cfg.Name)
		if name == model.ArchivedWorkflowDirName {
			return "", "", false, fmt.Errorf("archive target cannot be %s", model.ArchivedWorkflowDirName)
		}
		target = filepath.Join(rootDir, name)
		singleWorkflow = true
	}

	resolvedTarget, err := filepath.Abs(target)
	if err != nil {
		return "", "", false, fmt.Errorf("resolve archive target: %w", err)
	}
	info, err := os.Stat(resolvedTarget)
	if err != nil {
		return "", "", false, fmt.Errorf("stat archive target: %w", err)
	}
	if !info.IsDir() {
		return "", "", false, fmt.Errorf("archive target is not a directory: %s", resolvedTarget)
	}
	if pathContainsArchivedComponent(resolvedTarget) {
		return "", "", false, fmt.Errorf("archive target cannot be inside %s", model.ArchivedWorkflowDirName)
	}

	resolvedRoot := resolvedTarget
	if singleWorkflow {
		resolvedRoot = filepath.Dir(resolvedTarget)
	}

	return resolvedTarget, resolvedRoot, singleWorkflow, nil
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
