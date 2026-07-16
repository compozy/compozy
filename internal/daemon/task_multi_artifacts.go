package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	runparallel "github.com/compozy/compozy/internal/core/run/parallel"
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
	return copyTaskMultiArtifactTree(source, destination)
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

func copyTaskMultiArtifactTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("resolve workflow artifact path %s: %w", path, err)
		}
		if rel == "." {
			return nil
		}
		if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return fmt.Errorf("workflow artifact path %s escapes source %s", path, source)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat workflow artifact %s: %w", path, err)
		}
		mode := info.Mode()
		if mode&os.ModeSymlink != 0 {
			return fmt.Errorf("workflow artifact symlink %s is not supported", path)
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, dirPerm(mode.Perm()))
		}
		if !mode.IsRegular() {
			return fmt.Errorf("workflow artifact %s is not a regular file", path)
		}
		return copyTaskMultiArtifactFile(path, target, mode.Perm())
	})
}

func copyTaskMultiArtifactFile(source, destination string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), taskMultiWorktreeDirPerm); err != nil {
		return fmt.Errorf("create workflow artifact parent for %s: %w", destination, err)
	}
	if err := rejectTaskMultiArtifactDestinationSymlink(destination); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open workflow artifact %s: %w", source, err)
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, filePerm(mode))
	if err != nil {
		return fmt.Errorf("create workflow artifact %s: %w", destination, err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("copy workflow artifact %s to %s: %w", source, destination, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close workflow artifact %s: %w", destination, closeErr)
	}
	return nil
}

func rejectTaskMultiArtifactDestinationSymlink(path string) error {
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("workflow artifact destination %s is a symlink", path)
		}
		return nil
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("stat workflow artifact destination %s: %w", path, err)
	}
}

func dirPerm(mode os.FileMode) os.FileMode {
	// Guard against a zero FileMode creating unreadable 0000 directories.
	if mode == 0 {
		return taskMultiWorktreeDirPerm
	}
	return mode
}

func filePerm(mode os.FileMode) os.FileMode {
	// Guard against a zero FileMode creating unreadable 0000 files.
	if mode == 0 {
		return 0o600
	}
	return mode
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
	if err := copyTaskMultiArtifactFile(source, destination, info.Mode().Perm()); err != nil {
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
