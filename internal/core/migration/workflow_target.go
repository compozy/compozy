package migration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

var ErrInvalidWorkflowName = errors.New("invalid workflow name")

type workflowTargetOptions struct {
	command       string
	workspaceRoot string
	rootDir       string
	name          string
	tasksDir      string
	reviewsDir    string
	selectorFlags string
}

type workflowTargetResolution struct {
	target         string
	rootDir        string
	specificTarget bool
}

func resolveWorkflowTarget(cfg workflowTargetOptions) (workflowTargetResolution, error) {
	workflowName, err := normalizeWorkflowName(cfg.command, cfg.name)
	if err != nil {
		return workflowTargetResolution{}, err
	}

	specificTargets := 0
	if workflowName != "" {
		specificTargets++
	}
	if strings.TrimSpace(cfg.tasksDir) != "" {
		specificTargets++
	}
	if strings.TrimSpace(cfg.reviewsDir) != "" {
		specificTargets++
	}
	if specificTargets > 1 {
		return workflowTargetResolution{}, fmt.Errorf(
			"%s accepts only one of %s",
			cfg.command,
			cfg.selectorFlags,
		)
	}

	rootDir := strings.TrimSpace(cfg.rootDir)
	if rootDir == "" {
		rootDir = model.TasksBaseDirForWorkspace(cfg.workspaceRoot)
	}

	target := rootDir
	specificTarget := false
	switch {
	case strings.TrimSpace(cfg.reviewsDir) != "":
		target = strings.TrimSpace(cfg.reviewsDir)
		specificTarget = true
	case strings.TrimSpace(cfg.tasksDir) != "":
		target = strings.TrimSpace(cfg.tasksDir)
		specificTarget = true
	case workflowName != "":
		target = filepath.Join(rootDir, workflowName)
		specificTarget = true
	}

	resolvedTarget, err := filepath.Abs(target)
	if err != nil {
		return workflowTargetResolution{}, fmt.Errorf("resolve %s target: %w", cfg.command, err)
	}
	info, err := os.Stat(resolvedTarget)
	if err != nil {
		return workflowTargetResolution{}, fmt.Errorf("stat %s target: %w", cfg.command, err)
	}
	if !info.IsDir() {
		return workflowTargetResolution{}, fmt.Errorf(
			"%s target is not a directory: %s",
			cfg.command,
			resolvedTarget,
		)
	}

	resolvedRoot := resolvedTarget
	if specificTarget {
		resolvedRoot = filepath.Dir(resolvedTarget)
	}

	return workflowTargetResolution{
		target:         resolvedTarget,
		rootDir:        resolvedRoot,
		specificTarget: specificTarget,
	}, nil
}

func normalizeWorkflowName(command, name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", nil
	}
	if filepath.IsAbs(trimmed) || strings.ContainsAny(trimmed, `/\`) || !model.IsActiveWorkflowDirName(trimmed) {
		return "", fmt.Errorf(
			"%w: %s name must be a single active workflow directory name",
			ErrInvalidWorkflowName,
			command,
		)
	}
	return trimmed, nil
}
