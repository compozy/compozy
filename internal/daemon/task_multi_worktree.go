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
	"strings"
)

const (
	// taskMultiWorktreeStatusPreserved marks an allocated child worktree as kept
	// for manual review. V1 never deletes, merges, or pushes child worktrees.
	taskMultiWorktreeStatusPreserved = "preserved"
	// taskMultiWorktreeHeadRef is the symbolic ref resolved for the parent base.
	// `git rev-parse --abbrev-ref HEAD` returns this literal when the checkout is
	// detached, which the allocator rejects.
	taskMultiWorktreeHeadRef = "HEAD"

	// taskMultiWorktreeHashLen bounds the workspace-root hash segment so paths
	// stay short enough for local daemon/socket path constraints.
	taskMultiWorktreeHashLen = 12
	// taskMultiWorktreeParentShortLen bounds the parent-run path segment.
	taskMultiWorktreeParentShortLen = 12
	// taskMultiWorktreeSlugMaxLen bounds the sanitized slug path segment.
	taskMultiWorktreeSlugMaxLen = 40
	// taskMultiWorktreeIndexPadWidth zero-pads the child index for stable sort
	// order in directory listings.
	taskMultiWorktreeIndexPadWidth = 2

	taskMultiWorktreeDirPerm os.FileMode = 0o750
)

// taskMultiWorktreeBase captures the parent workspace branch and commit resolved
// once per parent run. Child worktrees are created detached at Commit while
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
// and detached worktree creation for parallel multi-run child tasks. It never
// creates branches, merges, pushes, or removes worktrees: V1 preserves every
// allocated worktree for manual review.
type taskMultiWorktreeAllocator struct {
	worktreesRoot string
	run           taskMultiWorktreeGitRunner
}

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
	parent := sanitizeTaskMultiWorktreeSegment(spec.ParentRunID, taskMultiWorktreeParentShortLen)
	if parent == "" {
		return "", errors.New("daemon: worktree parent run id is required")
	}
	slug := sanitizeTaskMultiWorktreeSegment(spec.Slug, taskMultiWorktreeSlugMaxLen)
	if slug == "" {
		return "", fmt.Errorf("daemon: worktree slug %q is not a valid path segment", spec.Slug)
	}
	leaf := fmt.Sprintf("%0*d-%s", taskMultiWorktreeIndexPadWidth, spec.Index, slug)
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
	sum := sha256.Sum256([]byte(filepath.Clean(workspaceRoot)))
	return hex.EncodeToString(sum[:])[:taskMultiWorktreeHashLen]
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
