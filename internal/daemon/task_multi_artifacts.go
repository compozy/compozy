package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	runparallel "github.com/compozy/compozy/internal/core/run/parallel"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/core/worktree"
)

const taskMultiTaskFilePattern = "task_%02d.md"

// mirrorTaskMultiWorkflowArtifacts mirrors the parent workflow artifact
// directory into a task worktree, overwriting regular files when they already
// exist. This keeps ignored .compozy/tasks workflows available to child task
// runs without making those runtime artifacts part of the Git merge surface.
func mirrorTaskMultiWorkflowArtifacts(sourceTaskDir, worktreeRoot, slug string) error {
	source := strings.TrimSpace(sourceTaskDir)
	if source == "" {
		return errors.New("daemon: workflow artifact source task directory is required")
	}
	root := strings.TrimSpace(worktreeRoot)
	if root == "" {
		return errors.New("daemon: workflow artifact destination worktree root is required")
	}
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return errors.New("daemon: workflow artifact destination slug is required")
	}
	destination := model.TaskDirectoryForWorkspace(root, trimmedSlug)
	if err := requireDirectory(source); err != nil {
		return fmt.Errorf("mirror workflow artifacts for %q: source task directory %s: %w", slug, source, err)
	}
	if err := requireTaskMultiArtifactDestination(destination); err != nil {
		return err
	}
	return worktree.OverlayTree(source, destination)
}

// mirrorTaskMultiGroupArtifacts copies the initiative specification tree, which
// includes the selected group's operational directory, into an isolated
// worktree. Task-group sync validates both sides of ExecutionScope against the
// child workspace, so copying only the leaf task directory would leave the PRD
// and TechSpec outside that workspace.
func mirrorTaskMultiGroupArtifacts(scope *model.ExecutionScope, worktreeRoot string) error {
	if scope == nil {
		return errors.New("daemon: task-group execution scope is required")
	}
	ref, err := taskgroups.ParseTaskGroupRef(strings.TrimSpace(scope.WorkflowRef))
	if err != nil {
		return err
	}
	source := strings.TrimSpace(scope.SpecDir)
	if source == "" {
		return errors.New("daemon: task-group specification directory is required")
	}
	root := strings.TrimSpace(worktreeRoot)
	if root == "" {
		return errors.New("daemon: task-group destination worktree root is required")
	}
	if err := requireDirectory(source); err != nil {
		return fmt.Errorf("mirror task-group artifacts for %q: specification directory %s: %w",
			ref.String(), source, err)
	}
	destination := model.TaskDirectoryForWorkspace(root, ref.Initiative)
	if err := requireTaskMultiArtifactDestination(destination); err != nil {
		return err
	}
	if err := worktree.OverlayTree(source, destination); err != nil {
		return err
	}
	operationalSource := strings.TrimSpace(scope.OperationalDir)
	if operationalSource == "" {
		return errors.New("daemon: task-group operational directory is required")
	}
	operationalDestination := model.TaskDirectoryForWorkspace(root, ref.String())
	if err := requireTaskMultiArtifactDestination(operationalDestination); err != nil {
		return err
	}
	return worktree.OverlayTree(operationalSource, operationalDestination)
}

func requireTaskMultiArtifactDestination(destination string) error {
	if strings.TrimSpace(destination) == "" {
		return errors.New("daemon: workflow artifact destination is required")
	}
	info, err := os.Lstat(destination)
	switch {
	case err == nil:
		if !info.IsDir() {
			return fmt.Errorf("workflow artifact destination %s exists and is not a directory", destination)
		}
		return nil
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("stat workflow artifact destination %s: %w", destination, err)
	}
}

func syncCompletedParallelTaskArtifacts(
	ctx context.Context,
	workspaceRoot string,
	tasks []runparallel.TaskOutcome,
	destinationsBySlug map[string]string,
) error {
	for index := range tasks {
		if err := ctx.Err(); err != nil {
			return err
		}
		task := tasks[index]
		if !task.Status.IsIntegrated() {
			continue
		}
		if err := syncParallelTaskArtifact(workspaceRoot, task, destinationsBySlug); err != nil {
			return err
		}
	}
	return nil
}

// syncParallelTaskArtifact copies only the completed task's canonical
// task_NN.md file back to the parent workflow directory. The launch mirror copies
// the whole ignored workflow artifact tree because prompts, ADRs, memory, and
// sibling task files can be runtime inputs; write-back stays intentionally narrow
// so a child task cannot overwrite shared workflow support files.
func syncParallelTaskArtifact(
	workspaceRoot string,
	task runparallel.TaskOutcome,
	destinationsBySlug map[string]string,
) error {
	if task.Task.Number <= 0 {
		return fmt.Errorf("sync task artifact for %s: invalid task number %d", task.Task.ID, task.Task.Number)
	}
	slug := strings.TrimSpace(task.Task.Slug)
	if slug == "" {
		return fmt.Errorf("sync task artifact for %s: task slug is required", task.Task.ID)
	}
	worktreePath := strings.TrimSpace(task.WorktreePath)
	if worktreePath == "" {
		return fmt.Errorf("sync task artifact for %s: worktree path is required", task.Task.ID)
	}
	fileName := fmt.Sprintf(taskMultiTaskFilePattern, task.Task.Number)
	destinationDir := taskArtifactDestinationDir(workspaceRoot, slug, destinationsBySlug)
	hasDestination, err := taskMultiArtifactDirectoryExists(destinationDir)
	if err != nil {
		return fmt.Errorf("sync task artifact %s to parent workspace: %w", fileName, err)
	}
	if !hasDestination {
		return nil
	}
	source := filepath.Join(model.TaskDirectoryForWorkspace(worktreePath, slug), fileName)
	destination := filepath.Join(destinationDir, fileName)
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("sync task artifact %s from %s: %w", fileName, worktreePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("sync task artifact %s from %s: symlinks are not supported", fileName, source)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("sync task artifact %s from %s: source is not a regular file", fileName, source)
	}
	if err := worktree.OverlayFile(source, destination, info.Mode().Perm()); err != nil {
		return fmt.Errorf("sync task artifact %s to parent workspace: %w", fileName, err)
	}
	return nil
}

func taskArtifactDestinationDir(workspaceRoot string, slug string, destinationsBySlug map[string]string) string {
	if destinationsBySlug != nil {
		if destination := strings.TrimSpace(destinationsBySlug[strings.TrimSpace(slug)]); destination != "" {
			return destination
		}
	}
	return model.TaskDirectoryForWorkspace(strings.TrimSpace(workspaceRoot), strings.TrimSpace(slug))
}

func taskMultiArtifactDirectoryExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if !info.IsDir() {
			return false, fmt.Errorf("workflow artifact directory %s is not a directory", path)
		}
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("stat workflow artifact directory %s: %w", path, err)
	}
}
