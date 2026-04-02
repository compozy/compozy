package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	specificTargets := 0
	if strings.TrimSpace(cfg.Name) != "" {
		specificTargets++
	}
	if strings.TrimSpace(cfg.TasksDir) != "" {
		specificTargets++
	}
	if specificTargets > 1 {
		return "", false, errors.New("sync accepts only one of --name or --tasks-dir")
	}

	rootDir := strings.TrimSpace(cfg.RootDir)
	if rootDir == "" {
		rootDir = model.TasksBaseDir()
	}

	target := rootDir
	singleWorkflow := false
	switch {
	case strings.TrimSpace(cfg.TasksDir) != "":
		target = strings.TrimSpace(cfg.TasksDir)
		singleWorkflow = true
	case strings.TrimSpace(cfg.Name) != "":
		target = filepath.Join(rootDir, strings.TrimSpace(cfg.Name))
		singleWorkflow = true
	}

	resolved, err := filepath.Abs(target)
	if err != nil {
		return "", false, fmt.Errorf("resolve sync target: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", false, fmt.Errorf("stat sync target: %w", err)
	}
	if !info.IsDir() {
		return "", false, fmt.Errorf("sync target is not a directory: %s", resolved)
	}
	return resolved, singleWorkflow, nil
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
