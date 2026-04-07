package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/tasks"
)

func syncTaskMetadata(ctx context.Context, cfg SyncConfig) (*SyncResult, error) {
	target, singleWorkflow, err := resolveSyncTarget(cfg)
	result := &SyncResult{Target: target}
	if err != nil {
		return result, err
	}

	if singleWorkflow {
		if err := syncWorkflow(target, result); err != nil {
			return result, err
		}
		sort.Strings(result.SyncedPaths)
		return result, nil
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return result, fmt.Errorf("read sync target: %w", err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if !entry.IsDir() || !model.IsActiveWorkflowDirName(entry.Name()) {
			continue
		}
		if err := syncWorkflow(filepath.Join(target, entry.Name()), result); err != nil {
			return result, err
		}
	}

	sort.Strings(result.SyncedPaths)
	return result, nil
}

func resolveSyncTarget(cfg SyncConfig) (string, bool, error) {
	resolved, err := resolveWorkflowTarget(workflowTargetOptions{
		command:       "sync",
		workspaceRoot: cfg.WorkspaceRoot,
		rootDir:       cfg.RootDir,
		name:          cfg.Name,
		tasksDir:      cfg.TasksDir,
		selectorFlags: "--name or --tasks-dir",
	})
	if err != nil {
		return "", false, err
	}
	return resolved.target, resolved.specificTarget, nil
}

func syncWorkflow(tasksDir string, result *SyncResult) error {
	if result == nil {
		return errors.New("sync result is required")
	}

	_, statErr := os.Stat(tasks.MetaPath(tasksDir))
	metaExisted := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat task meta for %s: %w", tasksDir, statErr)
	}

	if _, err := tasks.RefreshTaskMeta(tasksDir); err != nil {
		return err
	}

	result.WorkflowsScanned++
	if metaExisted {
		result.MetaUpdated++
	} else {
		result.MetaCreated++
	}
	result.SyncedPaths = append(result.SyncedPaths, tasksDir)
	return nil
}
