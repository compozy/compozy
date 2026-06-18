package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	// taskMultiWorktreeStatusPreserved marks an allocated child worktree as kept
	// for manual review when a run does not finalize cleanup.
	taskMultiWorktreeStatusPreserved = "preserved"
	// taskMultiWorktreeHeadRef is the symbolic ref resolved for the parent base.
	// `git rev-parse --abbrev-ref HEAD` returns this literal when the checkout is
	// detached, which the allocator rejects.
	taskMultiWorktreeHeadRef = "HEAD"

	// taskMultiWorktreeHashLen bounds the workspace-root hash segment so paths
	// stay short enough for local daemon/socket path constraints.
	taskMultiWorktreeHashLen = 12
	// taskMultiWorktreeParentShortLen bounds the readable parent-run path segment.
	taskMultiWorktreeParentShortLen = 12
	// taskMultiWorktreeParentHashLen sizes the digest suffix appended to the
	// parent-run segment. Generated run ids share a long common prefix, so a
	// digest of the full id keeps distinct parent runs in distinct directories.
	taskMultiWorktreeParentHashLen = 8
	// taskMultiWorktreeSlugMaxLen bounds the sanitized slug path segment.
	taskMultiWorktreeSlugMaxLen = 40
	// taskMultiWorktreeIndexPadWidth zero-pads the child index for stable sort
	// order in directory listings.
	taskMultiWorktreeIndexPadWidth = 2

	taskMultiWorktreeDirPerm os.FileMode = 0o750
)

// ConflictSet describes the unmerged files produced by a squash merge.
type ConflictSet struct {
	Files []string
	Clean bool
}

// WorktreeLifecycle is the narrow git boundary for write-back operations.
type WorktreeLifecycle interface {
	Commit(ctx context.Context, path string, message string) (string, error)
	CreateIntegrationBranch(
		ctx context.Context,
		workspaceRoot string,
		integrationPath string,
		integrationBranch string,
		baseRef string,
	) error
	SquashMerge(ctx context.Context, integrationPath string, worktreeRef string, message string) (ConflictSet, error)
	FastForward(ctx context.Context, workspaceRoot string, targetBranch string, integrationBranch string) error
	DiscardIntegrationBranch(
		ctx context.Context,
		workspaceRoot string,
		integrationPath string,
		integrationBranch string,
	) error
	Remove(ctx context.Context, workspaceRoot string, path string) error
	Prune(ctx context.Context, workspaceRoot string) error
}

// taskMultiWorktreeBase captures the parent workspace branch and commit resolved
// once per parent run. Child worktrees are created detached at Base.Commit while
// recording Branch for manual recombination.
type taskMultiWorktreeBase struct {
	Branch string
	Commit string
}

// taskMultiWorktreeSpec identifies one child worktree to allocate.
type taskMultiWorktreeSpec struct {
	WorkspaceRoot string
	ParentRunID   string
	Slug          string
	Index         int
	TaskNumber    int
	Base          taskMultiWorktreeBase
}

// taskMultiWorktreeAllocation is the per-child worktree metadata returned to the
// scheduler for parent-event emission and snapshot reconstruction. The fields
// mirror apicore.TaskRunMultipleItem and kinds.TaskRunMultiplePayload worktree
// metadata.
type taskMultiWorktreeAllocation struct {
	Path           string
	BaseBranch     string
	BaseCommit     string
	WorktreeStatus string
}

type taskMultiWorktreeGitRunner func(ctx context.Context, dir string, args ...string) (string, error)

// taskMultiWorktreeAllocator owns git worktree path planning, base resolution,
// detached worktree creation, and write-back lifecycle operations for parallel
// task execution.
type taskMultiWorktreeAllocator struct {
	worktreesRoot string
	run           taskMultiWorktreeGitRunner
}

var _ WorktreeLifecycle = (*taskMultiWorktreeAllocator)(nil)

// newTaskMultiWorktreeAllocator builds an allocator rooted at worktreesRoot, the
// home-scoped worktrees state directory (config.HomePaths.WorktreesDir).
func newTaskMultiWorktreeAllocator(worktreesRoot string) *taskMultiWorktreeAllocator {
	return &taskMultiWorktreeAllocator{
		worktreesRoot: strings.TrimSpace(worktreesRoot),
		run:           runTaskMultiWorktreeGitCommand,
	}
}

// ResolveBase resolves the parent workspace current branch and HEAD commit once.
// It rejects a detached parent checkout because each child worktree must record a
// named source branch for later manual recombination.
func (a *taskMultiWorktreeAllocator) ResolveBase(
	ctx context.Context,
	workspaceRoot string,
) (taskMultiWorktreeBase, error) {
	if a == nil || a.run == nil {
		return taskMultiWorktreeBase{}, errors.New("daemon: worktree allocator git runner is required")
	}
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return taskMultiWorktreeBase{}, errors.New("daemon: worktree workspace root is required")
	}
	branch, err := a.run(ctx, root, "rev-parse", "--abbrev-ref", taskMultiWorktreeHeadRef)
	if err != nil {
		return taskMultiWorktreeBase{}, fmt.Errorf("resolve parent branch in %s: %w", root, err)
	}
	branch = strings.TrimSpace(branch)
	if branch == "" || branch == taskMultiWorktreeHeadRef {
		return taskMultiWorktreeBase{}, fmt.Errorf(
			"parent workspace %s is on a detached HEAD; a named branch is required for parallel multi-run",
			root,
		)
	}
	commit, err := a.run(ctx, root, "rev-parse", taskMultiWorktreeHeadRef)
	if err != nil {
		return taskMultiWorktreeBase{}, fmt.Errorf("resolve parent head commit in %s: %w", root, err)
	}
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return taskMultiWorktreeBase{}, fmt.Errorf("parent workspace %s has no resolvable HEAD commit", root)
	}
	return taskMultiWorktreeBase{Branch: branch, Commit: commit}, nil
}

// Allocate creates one detached git worktree for a child task at the resolved
// base commit and returns its metadata. The target path is deterministic for a
// parent run and child index; an existing target path is reported as a clear
// collision error instead of being reused.
func (a *taskMultiWorktreeAllocator) Allocate(
	ctx context.Context,
	spec taskMultiWorktreeSpec,
) (taskMultiWorktreeAllocation, error) {
	if a == nil || a.run == nil {
		return taskMultiWorktreeAllocation{}, errors.New("daemon: worktree allocator git runner is required")
	}
	commit := strings.TrimSpace(spec.Base.Commit)
	if commit == "" {
		return taskMultiWorktreeAllocation{}, errors.New("daemon: worktree base commit is required")
	}
	path, err := planTaskMultiWorktreePath(a.worktreesRoot, spec)
	if err != nil {
		return taskMultiWorktreeAllocation{}, err
	}
	if err := ensureTaskMultiWorktreeTargetFree(path); err != nil {
		return taskMultiWorktreeAllocation{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), taskMultiWorktreeDirPerm); err != nil {
		return taskMultiWorktreeAllocation{}, fmt.Errorf("create worktree parent directory for %s: %w", path, err)
	}
	if _, err := a.run(ctx, spec.WorkspaceRoot, "worktree", "add", "--detach", path, commit); err != nil {
		return taskMultiWorktreeAllocation{}, fmt.Errorf("git worktree add --detach %s %s: %w", path, commit, err)
	}
	return taskMultiWorktreeAllocation{
		Path:           path,
		BaseBranch:     strings.TrimSpace(spec.Base.Branch),
		BaseCommit:     commit,
		WorktreeStatus: taskMultiWorktreeStatusPreserved,
	}, nil
}

// Commit captures residual changes in a worktree and returns the resulting HEAD.
// A clean worktree is a no-op that returns the current HEAD commit.
func (a *taskMultiWorktreeAllocator) Commit(
	ctx context.Context,
	path string,
	message string,
) (string, error) {
	run, err := a.requireGitRunner()
	if err != nil {
		return "", err
	}
	worktreePath, err := requireTaskMultiWorktreeValue(path, "worktree path")
	if err != nil {
		return "", err
	}
	commitMessage, err := requireTaskMultiWorktreeValue(message, "commit message")
	if err != nil {
		return "", err
	}
	status, err := run(ctx, worktreePath, "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("inspect worktree status in %s: %w", worktreePath, err)
	}
	if conflicts := taskMultiWorktreeUnmergedFiles(status); len(conflicts) > 0 {
		return "", fmt.Errorf(
			"worktree %s has unresolved merge conflicts: %s",
			worktreePath,
			strings.Join(conflicts, ", "),
		)
	}
	if strings.TrimSpace(status) == "" {
		return a.worktreeHead(ctx, worktreePath)
	}
	if _, err := run(ctx, worktreePath, "add", "-A"); err != nil {
		return "", fmt.Errorf("stage residual changes in %s: %w", worktreePath, err)
	}
	status, err = run(ctx, worktreePath, "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("inspect staged residual changes in %s: %w", worktreePath, err)
	}
	if conflicts := taskMultiWorktreeUnmergedFiles(status); len(conflicts) > 0 {
		return "", fmt.Errorf(
			"worktree %s has unresolved merge conflicts: %s",
			worktreePath,
			strings.Join(conflicts, ", "),
		)
	}
	if strings.TrimSpace(status) == "" {
		return a.worktreeHead(ctx, worktreePath)
	}
	if _, err := run(ctx, worktreePath, "commit", "-m", commitMessage); err != nil {
		return "", fmt.Errorf("commit residual changes in %s: %w", worktreePath, err)
	}
	return a.worktreeHead(ctx, worktreePath)
}

// CreateIntegrationBranch creates a dedicated integration worktree checked out
// to a new branch at baseRef, leaving the user's workspace branch untouched.
func (a *taskMultiWorktreeAllocator) CreateIntegrationBranch(
	ctx context.Context,
	workspaceRoot string,
	integrationPath string,
	integrationBranch string,
	baseRef string,
) error {
	run, err := a.requireGitRunner()
	if err != nil {
		return err
	}
	workspace, err := requireTaskMultiWorktreeValue(workspaceRoot, "workspace root")
	if err != nil {
		return err
	}
	path, err := requireTaskMultiWorktreeValue(integrationPath, "integration path")
	if err != nil {
		return err
	}
	branch, err := requireTaskMultiWorktreeValue(integrationBranch, "integration branch")
	if err != nil {
		return err
	}
	base, err := requireTaskMultiWorktreeValue(baseRef, "integration base ref")
	if err != nil {
		return err
	}
	if err := ensureTaskMultiWorktreeTargetFree(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), taskMultiWorktreeDirPerm); err != nil {
		return fmt.Errorf("create integration worktree parent directory for %s: %w", path, err)
	}
	if _, err := run(ctx, workspace, "worktree", "add", "-b", branch, path, base); err != nil {
		return fmt.Errorf("create integration branch %s at %s: %w", branch, base, err)
	}
	return nil
}

// SquashMerge stages worktreeRef into the integration worktree and commits one
// squash commit when clean. Conflicts are returned as data for the resolver.
func (a *taskMultiWorktreeAllocator) SquashMerge(
	ctx context.Context,
	integrationPath string,
	worktreeRef string,
	message string,
) (ConflictSet, error) {
	run, err := a.requireGitRunner()
	if err != nil {
		return ConflictSet{}, err
	}
	path, err := requireTaskMultiWorktreeValue(integrationPath, "integration path")
	if err != nil {
		return ConflictSet{}, err
	}
	ref, err := requireTaskMultiWorktreeValue(worktreeRef, "worktree ref")
	if err != nil {
		return ConflictSet{}, err
	}
	commitMessage, err := requireTaskMultiWorktreeValue(message, "commit message")
	if err != nil {
		return ConflictSet{}, err
	}
	status, err := run(ctx, path, "status", "--porcelain")
	if err != nil {
		return ConflictSet{}, fmt.Errorf("inspect integration status in %s: %w", path, err)
	}
	if strings.TrimSpace(status) != "" {
		if conflicts := taskMultiWorktreeUnmergedFiles(status); len(conflicts) > 0 {
			return ConflictSet{Files: conflicts, Clean: false}, nil
		}
		return ConflictSet{}, fmt.Errorf("integration worktree %s is dirty before squash merge", path)
	}
	if _, err := run(ctx, path, "merge", "--squash", "--", ref); err != nil {
		status, statusErr := run(ctx, path, "status", "--porcelain")
		if statusErr != nil {
			return ConflictSet{}, errors.Join(
				fmt.Errorf("squash merge %s into %s: %w", ref, path, err),
				fmt.Errorf("inspect conflict status in %s: %w", path, statusErr),
			)
		}
		if conflicts := taskMultiWorktreeUnmergedFiles(status); len(conflicts) > 0 {
			return ConflictSet{Files: conflicts, Clean: false}, nil
		}
		return ConflictSet{}, fmt.Errorf("squash merge %s into %s: %w", ref, path, err)
	}
	if _, err := run(ctx, path, "commit", "--allow-empty", "-m", commitMessage); err != nil {
		return ConflictSet{}, fmt.Errorf("commit squash merge for %s in %s: %w", ref, path, err)
	}
	return ConflictSet{Clean: true}, nil
}

// FastForward advances the checked-out target branch to the integration branch.
func (a *taskMultiWorktreeAllocator) FastForward(
	ctx context.Context,
	workspaceRoot string,
	targetBranch string,
	integrationBranch string,
) error {
	run, err := a.requireGitRunner()
	if err != nil {
		return err
	}
	workspace, err := requireTaskMultiWorktreeValue(workspaceRoot, "workspace root")
	if err != nil {
		return err
	}
	target, err := requireTaskMultiWorktreeValue(targetBranch, "target branch")
	if err != nil {
		return err
	}
	integration, err := requireTaskMultiWorktreeValue(integrationBranch, "integration branch")
	if err != nil {
		return err
	}
	current, err := run(ctx, workspace, "rev-parse", "--abbrev-ref", taskMultiWorktreeHeadRef)
	if err != nil {
		return fmt.Errorf("inspect current branch in %s: %w", workspace, err)
	}
	current = strings.TrimSpace(current)
	if current != target {
		return fmt.Errorf("workspace %s is on branch %q, want target branch %q", workspace, current, target)
	}
	status, err := run(ctx, workspace, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("inspect target branch status in %s: %w", workspace, err)
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("target branch %s in %s must be clean before fast-forward", target, workspace)
	}
	if _, err := run(ctx, workspace, "merge-base", "--is-ancestor", target, integration); err != nil {
		return fmt.Errorf("integration branch %s is not a fast-forward of %s: %w", integration, target, err)
	}
	if _, err := run(ctx, workspace, "merge", "--ff-only", integration); err != nil {
		return fmt.Errorf("fast-forward %s to %s: %w", target, integration, err)
	}
	return nil
}

// DiscardIntegrationBranch removes the integration worktree and deletes the
// integration branch after a failed or completed run.
func (a *taskMultiWorktreeAllocator) DiscardIntegrationBranch(
	ctx context.Context,
	workspaceRoot string,
	integrationPath string,
	integrationBranch string,
) error {
	run, err := a.requireGitRunner()
	if err != nil {
		return err
	}
	workspace, err := requireTaskMultiWorktreeValue(workspaceRoot, "workspace root")
	if err != nil {
		return err
	}
	path, err := requireTaskMultiWorktreeValue(integrationPath, "integration path")
	if err != nil {
		return err
	}
	branch, err := requireTaskMultiWorktreeValue(integrationBranch, "integration branch")
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(path); statErr == nil {
		// Discarding the integration branch intentionally drops its in-progress
		// index, including conflict state. Task worktrees use Remove without force.
		if _, err := run(ctx, workspace, "worktree", "remove", "--force", path); err != nil {
			return fmt.Errorf("remove integration worktree %s: %w", path, err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat integration worktree %s: %w", path, statErr)
	}
	if _, err := run(ctx, workspace, "branch", "-D", branch); err != nil {
		return fmt.Errorf("delete integration branch %s: %w", branch, err)
	}
	return nil
}

// Remove removes a clean task worktree without forcing away uncommitted changes.
func (a *taskMultiWorktreeAllocator) Remove(ctx context.Context, workspaceRoot string, path string) error {
	run, err := a.requireGitRunner()
	if err != nil {
		return err
	}
	workspace, err := requireTaskMultiWorktreeValue(workspaceRoot, "workspace root")
	if err != nil {
		return err
	}
	worktreePath, err := requireTaskMultiWorktreeValue(path, "worktree path")
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(worktreePath); errors.Is(statErr, os.ErrNotExist) {
		return nil
	} else if statErr != nil {
		return fmt.Errorf("stat worktree %s: %w", worktreePath, statErr)
	}
	if _, err := run(ctx, workspace, "worktree", "remove", worktreePath); err != nil {
		return fmt.Errorf("remove worktree %s: %w", worktreePath, err)
	}
	return nil
}

// Prune removes stale git worktree administrative references.
func (a *taskMultiWorktreeAllocator) Prune(ctx context.Context, workspaceRoot string) error {
	run, err := a.requireGitRunner()
	if err != nil {
		return err
	}
	workspace, err := requireTaskMultiWorktreeValue(workspaceRoot, "workspace root")
	if err != nil {
		return err
	}
	if _, err := run(ctx, workspace, "worktree", "prune"); err != nil {
		return fmt.Errorf("prune worktrees for %s: %w", workspace, err)
	}
	return nil
}

// Head resolves HEAD in the supplied worktree path.
func (a *taskMultiWorktreeAllocator) Head(ctx context.Context, path string) (string, error) {
	worktreePath, err := requireTaskMultiWorktreeValue(path, "worktree path")
	if err != nil {
		return "", err
	}
	return a.worktreeHead(ctx, worktreePath)
}

// planTaskMultiWorktreePath builds the deterministic, sanitized worktree path:
//
//	<worktreesRoot>/<workspace-hash>/<parent-short>/<NN-slug>
//
// The path is parent-run scoped so repeated batches never collide and is kept
// short enough to avoid local daemon/socket path-length problems.
func planTaskMultiWorktreePath(worktreesRoot string, spec taskMultiWorktreeSpec) (string, error) {
	root := strings.TrimSpace(worktreesRoot)
	if root == "" {
		return "", errors.New("daemon: worktree allocator root is required")
	}
	workspaceRoot := strings.TrimSpace(spec.WorkspaceRoot)
	if workspaceRoot == "" {
		return "", errors.New("daemon: worktree workspace root is required")
	}
	if spec.Index < 0 {
		return "", fmt.Errorf("daemon: worktree index must be non-negative, got %d", spec.Index)
	}
	if spec.TaskNumber < 0 {
		return "", fmt.Errorf("daemon: worktree task number must be non-negative, got %d", spec.TaskNumber)
	}
	parent := sanitizeTaskMultiWorktreeSegment(spec.ParentRunID, taskMultiWorktreeParentShortLen)
	if parent == "" {
		return "", errors.New("daemon: worktree parent run id is required")
	}
	// Generated run ids share a long "task-multi-<date>-..." prefix, so the
	// truncated readable segment alone is not unique. Append a digest of the full
	// run id so distinct parent runs never resolve to the same preserved worktree
	// directory, which would otherwise surface as a "target already exists" error.
	parent += "-" + taskMultiShortHash(strings.TrimSpace(spec.ParentRunID), taskMultiWorktreeParentHashLen)
	slug := sanitizeTaskMultiWorktreeSegment(spec.Slug, taskMultiWorktreeSlugMaxLen)
	if slug == "" {
		return "", fmt.Errorf("daemon: worktree slug %q is not a valid path segment", spec.Slug)
	}
	leafNumber := spec.Index
	if spec.TaskNumber > 0 {
		leafNumber = spec.TaskNumber
	}
	leaf := fmt.Sprintf("%0*d-%s", taskMultiWorktreeIndexPadWidth, leafNumber, slug)
	return filepath.Join(root, taskMultiWorkspaceHash(workspaceRoot), parent, leaf), nil
}

// sanitizeTaskMultiWorktreeSegment lowercases value and reduces it to a safe
// filesystem segment: ASCII letters, digits, and underscores are preserved while
// every other rune (spaces, path separators, dots) collapses to a single dash.
// Leading and trailing dashes are trimmed and the result is capped to maxLen.
func sanitizeTaskMultiWorktreeSegment(value string, maxLen int) string {
	lowered := strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	b.Grow(len(lowered))
	lastDash := false
	for _, r := range lowered {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	sanitized := strings.Trim(b.String(), "-")
	if maxLen > 0 && len(sanitized) > maxLen {
		sanitized = strings.Trim(sanitized[:maxLen], "-")
	}
	return sanitized
}

// taskMultiWorkspaceHash derives a short stable digest of the original workspace
// root so worktrees from different checkouts never share a parent directory.
func taskMultiWorkspaceHash(workspaceRoot string) string {
	return taskMultiShortHash(filepath.Clean(workspaceRoot), taskMultiWorktreeHashLen)
}

// taskMultiShortHash returns the first n hex characters of the SHA-256 digest of
// value, used for short, stable, collision-resistant path segments.
func taskMultiShortHash(value string, n int) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:n]
}

func ensureTaskMultiWorktreeTargetFree(path string) error {
	switch _, err := os.Stat(path); {
	case err == nil:
		return fmt.Errorf("worktree target already exists: %s", path)
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("stat worktree target %s: %w", path, err)
	}
}

func (a *taskMultiWorktreeAllocator) requireGitRunner() (taskMultiWorktreeGitRunner, error) {
	if a == nil || a.run == nil {
		return nil, errors.New("daemon: worktree allocator git runner is required")
	}
	return a.run, nil
}

func (a *taskMultiWorktreeAllocator) worktreeHead(ctx context.Context, path string) (string, error) {
	run, err := a.requireGitRunner()
	if err != nil {
		return "", err
	}
	head, err := run(ctx, path, "rev-parse", taskMultiWorktreeHeadRef)
	if err != nil {
		return "", fmt.Errorf("resolve worktree head in %s: %w", path, err)
	}
	head = strings.TrimSpace(head)
	if head == "" {
		return "", fmt.Errorf("worktree %s has no resolvable HEAD commit", path)
	}
	return head, nil
}

func requireTaskMultiWorktreeValue(value string, name string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("daemon: %s is required", name)
	}
	return trimmed, nil
}

func taskMultiWorktreeUnmergedFiles(status string) []string {
	seen := make(map[string]struct{})
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 3 {
			continue
		}
		code := line[:2]
		if !taskMultiWorktreeStatusCodeUnmerged(code) {
			continue
		}
		path := taskMultiWorktreeStatusPath(line[3:])
		if path == "" {
			continue
		}
		seen[path] = struct{}{}
	}
	files := make([]string, 0, len(seen))
	for file := range seen {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func taskMultiWorktreeStatusCodeUnmerged(code string) bool {
	switch code {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return true
	default:
		return false
	}
}

func taskMultiWorktreeStatusPath(raw string) string {
	path := strings.TrimSpace(raw)
	if _, after, ok := strings.Cut(path, " -> "); ok {
		path = after
	}
	if unquoted, err := strconv.Unquote(path); err == nil {
		path = unquoted
	}
	return path
}

func runTaskMultiWorktreeGitCommand(ctx context.Context, dir string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", strings.TrimSpace(dir)}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message != "" {
			return "", fmt.Errorf("%w: %s", err, message)
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}
