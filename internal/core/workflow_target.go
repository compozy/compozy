package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

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
	specificTargets := 0
	if strings.TrimSpace(cfg.name) != "" {
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
	case strings.TrimSpace(cfg.name) != "":
		target = filepath.Join(rootDir, strings.TrimSpace(cfg.name))
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
