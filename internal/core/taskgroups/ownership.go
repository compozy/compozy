package taskgroups

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/tasks"
)

// TaskGroupManifest contains qualified task ownership declared by one task group manifest.
type TaskGroupManifest struct {
	TaskGroupID string
	TaskIDs     []string
}

// QualifiedTaskID returns the initiative-wide identity of a task-group-local task.
func QualifiedTaskID(taskGroupID, taskID string) string {
	return strings.TrimSpace(taskGroupID) + "/" + strings.TrimSpace(taskID)
}

// ValidateTaskGroupManifest rejects escaped task paths before parsing task files.
func ValidateTaskGroupManifest(
	ctx context.Context,
	tasksDir string,
	workflowRef string,
	taskGroupID string,
) (TaskGroupManifest, []Issue, error) {
	if err := context.Cause(ctx); err != nil {
		return TaskGroupManifest{}, nil, fmt.Errorf("validate task group manifest: %w", err)
	}
	manifest, err := tasks.ReadTaskGraphManifest(tasksDir)
	if err != nil {
		return TaskGroupManifest{}, nil, err
	}
	issues := validateManifestTaskPaths(tasksDir, taskGroupID, manifest)
	if len(issues) > 0 {
		return TaskGroupManifest{}, sortedIssues(issues), nil
	}
	_, taskFiles, err := tasks.LoadValidatedTaskGraphManifest(ctx, tasksDir, workflowRef)
	if err != nil {
		return TaskGroupManifest{}, nil, err
	}
	if len(taskFiles) == 0 {
		issues = append(issues, Issue{Path: manifest.Path, Field: "graph.nodes", Message: "no executable tasks"})
	}
	result := TaskGroupManifest{TaskGroupID: taskGroupID, TaskIDs: make([]string, 0, len(taskFiles))}
	for index := range taskFiles {
		taskFile := &taskFiles[index]
		result.TaskIDs = append(result.TaskIDs, QualifiedTaskID(taskGroupID, taskFile.ID))
	}
	slices.Sort(result.TaskIDs)
	return result, sortedIssues(issues), nil
}

// ValidateTaskGroupManifestContainment rejects a task group graph that resolves a
// node outside the selected task group. TaskGroups without a graph manifest retain
// the legacy flat-task execution path.
func ValidateTaskGroupManifestContainment(ctx context.Context, target Target) error {
	if err := context.Cause(ctx); err != nil {
		return fmt.Errorf("validate task group manifest containment: %w", err)
	}
	if target.Mode != TargetModeTaskGroup {
		return newError(
			ErrSelectionRequired,
			target.Ref.Initiative,
			target.Ref.TaskGroupID,
			target.TaskGroupDir,
			[]Issue{{Field: "reference", Message: "a complete workflow target is required"}},
		)
	}
	manifest, err := tasks.ReadTaskGraphManifest(target.TasksDir)
	if errors.Is(err, tasks.ErrTaskGraphManifestMissing) {
		return nil
	}
	if err != nil {
		return err
	}
	issues := validateManifestTaskPaths(target.TasksDir, target.TaskGroup.ID, manifest)
	if len(issues) == 0 {
		return nil
	}
	return newError(ErrInvalidPlan, target.Ref.Initiative, target.TaskGroup.ID, manifest.Path, issues)
}

func validateManifestTaskPaths(tasksDir, taskGroupID string, manifest tasks.TaskGraphManifest) []Issue {
	root, err := filepath.Abs(tasksDir)
	if err != nil {
		return []Issue{{Path: manifest.Path, Field: "tasks_dir", Message: err.Error()}}
	}
	issues := make([]Issue, 0)
	for index, node := range manifest.Graph.Nodes {
		file := filepath.FromSlash(strings.TrimSpace(node.File))
		candidate := filepath.Join(root, file)
		if filepath.IsAbs(file) || !containedPath(root, candidate) {
			issues = append(issues, Issue{
				Path: manifest.Path, Field: fmt.Sprintf("graph.nodes[%d].file", index),
				Message: fmt.Sprintf("sibling-ownership violation for task group %s", taskGroupID),
			})
			continue
		}
		if _, err := os.Lstat(candidate); err == nil {
			resolved, resolveErr := filepath.EvalSymlinks(candidate)
			if resolveErr != nil || !containedPath(root, resolved) {
				issues = append(issues, Issue{
					Path: manifest.Path, Field: fmt.Sprintf("graph.nodes[%d].file", index),
					Message: fmt.Sprintf("sibling-ownership violation for task group %s", taskGroupID),
				})
			}
		}
	}
	return issues
}

func containedPath(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
		!filepath.IsAbs(relative)
}

// AuditTaskOwnership validates exclusive initiative-wide qualified task ownership.
func AuditTaskOwnership(plan Plan, expected []string, manifests []TaskGroupManifest) []Issue {
	owners, seenManifest, issues := auditDeclaredManifests(plan, manifests)
	issues = append(issues, auditMissingManifests(plan, seenManifest)...)
	issues = append(issues, auditDuplicateOwners(plan, owners)...)
	issues = append(issues, auditExpectedTasks(plan, expected, owners)...)
	return sortedIssues(issues)
}

func auditDeclaredManifests(plan Plan, manifests []TaskGroupManifest) (map[string][]string, map[string]bool, []Issue) {
	issues := make([]Issue, 0)
	owners := make(map[string][]string)
	seenManifest := make(map[string]bool, len(manifests))
	for _, manifest := range manifests {
		seenManifest[manifest.TaskGroupID] = true
		if _, exists := plan.TaskGroup(manifest.TaskGroupID); !exists {
			issues = append(
				issues,
				Issue{
					Path:    plan.Path,
					Field:   "ownership",
					Message: fmt.Sprintf("unknown owning task group %q", manifest.TaskGroupID),
				},
			)
		}
		if len(manifest.TaskIDs) == 0 {
			issues = append(
				issues,
				Issue{Path: plan.Path, Field: "ownership." + manifest.TaskGroupID, Message: "no executable tasks"},
			)
		}
		for _, taskID := range manifest.TaskIDs {
			if !strings.HasPrefix(taskID, manifest.TaskGroupID+"/") {
				issues = append(
					issues,
					Issue{
						Path:    plan.Path,
						Field:   "ownership." + manifest.TaskGroupID,
						Message: fmt.Sprintf("cross-task-group task reference %q", taskID),
					},
				)
			}
			owners[taskID] = append(owners[taskID], manifest.TaskGroupID)
		}
	}
	return owners, seenManifest, issues
}

func auditMissingManifests(plan Plan, seenManifest map[string]bool) []Issue {
	issues := make([]Issue, 0)
	for index := range plan.TaskGroups {
		taskGroup := &plan.TaskGroups[index]
		if !seenManifest[taskGroup.ID] {
			issues = append(
				issues,
				Issue{Path: plan.Path, Field: "ownership." + taskGroup.ID, Message: "no executable tasks"},
			)
		}
	}
	return issues
}

func auditDuplicateOwners(plan Plan, owners map[string][]string) []Issue {
	issues := make([]Issue, 0)
	for taskID, ownerIDs := range owners {
		if len(ownerIDs) < 2 {
			continue
		}
		slices.Sort(ownerIDs)
		issues = append(
			issues,
			Issue{
				Path:    plan.Path,
				Field:   "ownership",
				Message: fmt.Sprintf("task %q has duplicate owners %s", taskID, strings.Join(ownerIDs, ", ")),
			},
		)
	}
	return issues
}

func auditExpectedTasks(plan Plan, expected []string, owners map[string][]string) []Issue {
	issues := make([]Issue, 0)
	for _, taskID := range expected {
		taskGroupID, _, valid := strings.Cut(taskID, "/")
		if !valid || !taskGroupIDPattern.MatchString(taskGroupID) {
			issues = append(
				issues,
				Issue{Path: plan.Path, Field: "ownership", Message: fmt.Sprintf("task %q is not qualified", taskID)},
			)
			continue
		}
		ownerIDs := owners[taskID]
		slices.Sort(ownerIDs)
		switch len(ownerIDs) {
		case 0:
			issues = append(
				issues,
				Issue{Path: plan.Path, Field: "ownership", Message: fmt.Sprintf("unowned task %q", taskID)},
			)
		case 1:
			if ownerIDs[0] != taskGroupID {
				issues = append(
					issues,
					Issue{
						Path:    plan.Path,
						Field:   "ownership",
						Message: fmt.Sprintf("task %q is owned by %q", taskID, ownerIDs[0]),
					},
				)
			}
		}
	}
	return issues
}

func sortedIssues(issues []Issue) []Issue {
	issues = slices.Clone(issues)
	slices.SortFunc(issues, compareIssue)
	return issues
}
