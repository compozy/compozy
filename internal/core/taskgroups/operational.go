package taskgroups

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OperationalPaths identifies a task-group-owned directory without trusting the
// mutable Task Group plan. It is used only to preserve final-review evidence
// if the plan becomes unreadable before completion can be recorded.
type OperationalPaths struct {
	Ref           Ref
	InitiativeDir string
	TaskGroupDir  string
}

// ResolveOperationalPaths resolves a contained task group directory from a
// syntactically valid public reference. Completion still re-resolves the full
// Target before it can mutate the canonical plan.
func ResolveOperationalPaths(ctx context.Context, workspaceRoot, reference string) (OperationalPaths, error) {
	if err := context.Cause(ctx); err != nil {
		return OperationalPaths{}, fmt.Errorf("resolve task group operational paths: %w", err)
	}
	ref, err := ParseTaskGroupRef(reference)
	if err != nil {
		return OperationalPaths{}, err
	}
	tasksRoot, err := canonicalTasksRoot(workspaceRoot)
	if err != nil {
		return OperationalPaths{}, err
	}
	initiativeDir, err := resolveInitiative(tasksRoot, ref.Initiative)
	if err != nil {
		return OperationalPaths{}, err
	}
	taskGroupDir, err := resolveOperationalTaskGroupDirectory(initiativeDir, ref.TaskGroupID)
	if err != nil {
		return OperationalPaths{}, err
	}
	return OperationalPaths{Ref: ref, InitiativeDir: initiativeDir, TaskGroupDir: taskGroupDir}, nil
}

func resolveOperationalTaskGroupDirectory(initiativeDir, taskGroupID string) (string, error) {
	taskGroupsRoot := filepath.Join(initiativeDir, "_task_groups")
	entries, err := os.ReadDir(taskGroupsRoot)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read task group directory: %w", err)
		}
		return resolveTaskGroupDirectory(initiativeDir, TaskGroup{
			ID:        taskGroupID,
			Directory: "_task_groups/" + taskGroupID,
		})
	}

	candidates := make([]string, 0, 1)
	for _, entry := range entries {
		directory := "_task_groups/" + entry.Name()
		if validTaskGroupDirectory(taskGroupID, directory) {
			candidates = append(candidates, directory)
		}
	}
	switch len(candidates) {
	case 0:
		return resolveTaskGroupDirectory(initiativeDir, TaskGroup{
			ID:        taskGroupID,
			Directory: "_task_groups/" + taskGroupID,
		})
	case 1:
		return resolveTaskGroupDirectory(initiativeDir, TaskGroup{ID: taskGroupID, Directory: candidates[0]})
	default:
		return "", newError(
			ErrInvalidPlan,
			"",
			taskGroupID,
			"",
			[]Issue{{
				Path:    taskGroupsRoot,
				Field:   "task_group_directory",
				Message: "multiple directories match task group " + taskGroupID + ": " + strings.Join(candidates, ", "),
			}},
		)
	}
}
