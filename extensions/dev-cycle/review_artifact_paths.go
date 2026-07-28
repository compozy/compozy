package devcycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const reviewTasksDir = ".compozy/tasks"

var reviewTaskNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type reviewArtifactRoot struct {
	root     *os.Root
	taskName string
	taskDir  string
}

func openReviewArtifactRoot(
	scope *toolspkg.ExtensionToolWorkspaceScope,
	taskName string,
	createTask bool,
) (*reviewArtifactRoot, error) {
	if scope == nil || strings.TrimSpace(scope.ID) == "" || strings.TrimSpace(scope.Root) == "" {
		return nil, errors.New("dev-cycle: trusted workspace scope is required")
	}
	if err := validateReviewTaskName(taskName); err != nil {
		return nil, err
	}
	canonicalRoot, err := canonicalReviewWorkspaceRoot(scope.Root)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(canonicalRoot)
	if err != nil {
		return nil, fmt.Errorf("dev-cycle: open trusted workspace root: %w", err)
	}
	artifactRoot := &reviewArtifactRoot{
		root:     root,
		taskName: taskName,
		taskDir:  filepath.Join(reviewTasksDir, taskName),
	}
	if err := artifactRoot.validateAncestors(createTask); err != nil {
		closeErr := root.Close()
		if closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("dev-cycle: close trusted workspace root: %w", closeErr))
		}
		return nil, err
	}
	return artifactRoot, nil
}

func canonicalReviewWorkspaceRoot(value string) (string, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("dev-cycle: resolve trusted workspace root: %w", err)
	}
	linkInfo, err := os.Lstat(absolute)
	if err != nil {
		return "", fmt.Errorf("dev-cycle: inspect trusted workspace root: %w", err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("dev-cycle: trusted workspace root %q must not be a symlink", absolute)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("dev-cycle: resolve trusted workspace root symlinks: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("dev-cycle: inspect trusted workspace root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("dev-cycle: trusted workspace root %q is not a directory", resolved)
	}
	return filepath.Clean(resolved), nil
}

func validateReviewTaskName(taskName string) error {
	trimmed := strings.TrimSpace(taskName)
	if trimmed == "" || trimmed != taskName || !reviewTaskNamePattern.MatchString(trimmed) ||
		trimmed == "." || trimmed == ".." {
		return fmt.Errorf("dev-cycle: invalid review task name %q", taskName)
	}
	return nil
}

func (r *reviewArtifactRoot) validateAncestors(createTask bool) error {
	if err := requireRealReviewDirectory(r.root, ".compozy"); err != nil {
		return err
	}
	if err := requireRealReviewDirectory(r.root, reviewTasksDir); err != nil {
		return err
	}
	if createTask {
		if err := r.root.Mkdir(r.taskDir, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("dev-cycle: create review task directory %q: %w", r.taskName, err)
		}
	}
	if err := requireRealReviewDirectory(r.root, r.taskDir); err != nil {
		return err
	}
	return nil
}

func requireRealReviewDirectory(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if err != nil {
		return fmt.Errorf("dev-cycle: inspect review directory %q: %w", name, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("dev-cycle: review directory %q must be a real directory", name)
	}
	return nil
}

func (r *reviewArtifactRoot) close() error {
	if err := r.root.Close(); err != nil {
		return fmt.Errorf("dev-cycle: close trusted workspace root: %w", err)
	}
	return nil
}

func reviewRoundDirName(round int) string {
	return fmt.Sprintf("reviews-%03d", round)
}

func reviewIssueFileName(number int) string {
	return fmt.Sprintf("issue_%03d.md", number)
}
