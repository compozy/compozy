package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/core/gitenv"
	runparallel "github.com/compozy/compozy/internal/core/run/parallel"
)

const (
	// Worktree status values describe the actual lifecycle state surfaced through
	// task.multi and task.parallel payloads.
	taskMultiWorktreeStatusActive    = "active"
	taskMultiWorktreeStatusRemoved   = "removed"
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
	Files       []string
	StagedFiles []string
	Clean       bool
}

// WorktreeLifecycle is the narrow git boundary for write-back operations.
type WorktreeLifecycle interface {
	CommitTask(ctx context.Context, spec runparallel.TaskCommitSpec) (string, error)
	CommitStaged(ctx context.Context, spec runparallel.StagedCommitSpec) (string, error)
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
	ResultBranch  string
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
	WorktreeReason string
	ResultBranch   string
	NoChanges      bool
}

type taskMultiWorktreeGitRunner func(ctx context.Context, dir string, args ...string) (string, error)

// taskMultiWorktreeAllocator owns git worktree path planning, base resolution,
// detached worktree creation, and write-back lifecycle operations for parallel
// task execution.
type taskMultiWorktreeAllocator struct {
	worktreesRoot string
	run           taskMultiWorktreeGitRunner
	// git is not safe under concurrent worktree operations on one repository:
	// parallel `git worktree add`/`remove` race on the shared .git/worktrees
	// metadata (e.g. reading a sibling's commondir mid-write). A parallel wave
	// allocates and tears down one worktree per task at once, so serialize those
	// metadata operations per repository. Task execution stays parallel.
	locksMu sync.Mutex
	locks   map[string]*sync.Mutex
}

var _ WorktreeLifecycle = (*taskMultiWorktreeAllocator)(nil)

// newTaskMultiWorktreeAllocator builds an allocator rooted at worktreesRoot, the
// home-scoped worktrees state directory (config.HomePaths.WorktreesDir).
func newTaskMultiWorktreeAllocator(worktreesRoot string) *taskMultiWorktreeAllocator {
	return &taskMultiWorktreeAllocator{
		worktreesRoot: strings.TrimSpace(worktreesRoot),
		run:           runTaskMultiWorktreeGitCommand,
		locks:         make(map[string]*sync.Mutex),
	}
}

// lockWorktreeMeta serializes git worktree metadata operations for one repository
// and returns the unlock func. Different repositories proceed independently.
func (a *taskMultiWorktreeAllocator) lockWorktreeMeta(workspaceRoot string) func() {
	key := strings.TrimSpace(workspaceRoot)
	a.locksMu.Lock()
	if a.locks == nil {
		a.locks = make(map[string]*sync.Mutex)
	}
	mu, ok := a.locks[key]
	if !ok {
		mu = &sync.Mutex{}
		a.locks[key] = mu
	}
	a.locksMu.Unlock()
	mu.Lock()
	return mu.Unlock
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
	// Serialize this repository's worktree metadata reads/writes so a parallel
	// wave's concurrent allocations do not race on .git/worktrees.
	unlock := a.lockWorktreeMeta(spec.WorkspaceRoot)
	defer unlock()
	stalePaths, err := a.staleCompozyWorktreeRegistrations(ctx, spec.WorkspaceRoot)
	if err != nil {
		return taskMultiWorktreeAllocation{}, err
	}
	for _, stalePath := range stalePaths {
		slog.Default().Warn(
			"daemon: git worktree is registered outside the current Compozy home",
			"worktree_path",
			stalePath,
			"current_worktrees_root",
			a.worktreesRoot,
			"hint",
			"the worktree may belong to a previous COMPOZY_HOME and will not be deleted automatically",
		)
	}
	if err := os.MkdirAll(filepath.Dir(path), taskMultiWorktreeDirPerm); err != nil {
		return taskMultiWorktreeAllocation{}, fmt.Errorf("create worktree parent directory for %s: %w", path, err)
	}
	resultBranch := strings.TrimSpace(spec.ResultBranch)
	args := []string{"worktree", "add", "--detach", path, commit}
	if resultBranch != "" {
		args = []string{"worktree", "add", "-b", resultBranch, path, commit}
	}
	if _, err := a.run(ctx, spec.WorkspaceRoot, args...); err != nil {
		return taskMultiWorktreeAllocation{}, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return taskMultiWorktreeAllocation{
		Path:           path,
		BaseBranch:     strings.TrimSpace(spec.Base.Branch),
		BaseCommit:     commit,
		WorktreeStatus: taskMultiWorktreeStatusActive,
		ResultBranch:   resultBranch,
	}, nil
}

func (a *taskMultiWorktreeAllocator) staleCompozyWorktreeRegistrations(
	ctx context.Context,
	workspaceRoot string,
) ([]string, error) {
	run, err := a.requireGitRunner()
	if err != nil {
		return nil, err
	}
	workspace, err := requireTaskMultiWorktreeValue(workspaceRoot, "workspace root")
	if err != nil {
		return nil, err
	}
	registered, err := run(ctx, workspace, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("inspect registered worktrees before allocation: %w", err)
	}
	stale := make([]string, 0)
	for _, line := range strings.Split(registered, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "worktree ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "worktree "))
		if !looksLikeCompozyWorktreePath(path) {
			continue
		}
		_, owned, ownershipErr := cleanOwnedWorktreePath(a.worktreesRoot, path)
		if ownershipErr != nil {
			return nil, ownershipErr
		}
		if !owned {
			stale = append(stale, path)
		}
	}
	return stale, nil
}

func looksLikeCompozyWorktreePath(path string) bool {
	normalized := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	return strings.Contains(normalized, "/state/worktrees/")
}

// CommitTask commits only the task-produced paths recorded by the child run's
// worktree scope. A task with no produced paths is a no-op that returns HEAD.
func (a *taskMultiWorktreeAllocator) CommitTask(
	ctx context.Context,
	spec runparallel.TaskCommitSpec,
) (string, error) {
	run, err := a.requireGitRunner()
	if err != nil {
		return "", err
	}
	worktreePath, err := requireTaskMultiWorktreeValue(spec.Path, "worktree path")
	if err != nil {
		return "", err
	}
	commitMessage, err := requireTaskMultiWorktreeValue(spec.Message, "commit message")
	if err != nil {
		return "", err
	}
	if !spec.ScopeSupported {
		return "", runparallel.NewTaskCommitScopeError(fmt.Errorf(
			"task worktree %s missing supported produced-change scope %s: %s",
			worktreePath,
			strings.TrimSpace(spec.ScopeArtifactPath),
			strings.TrimSpace(spec.ScopeError),
		))
	}
	if len(spec.PreExistingChangedPaths) > 0 {
		return "", runparallel.NewTaskCommitScopeError(fmt.Errorf(
			"task worktree %s changed pre-existing dirty paths: %s",
			worktreePath,
			strings.Join(spec.PreExistingChangedPaths, ", "),
		))
	}
	paths, err := taskMultiNormalizeGitPaths(spec.ProducedPaths)
	if err != nil {
		return "", runparallel.NewTaskCommitScopeError(fmt.Errorf("validate task produced paths: %w", err))
	}
	if len(paths) == 0 {
		return a.worktreeHead(ctx, worktreePath)
	}
	return a.commitExplicitPaths(ctx, run, worktreePath, commitMessage, paths, paths)
}

// CommitStaged commits a constrained integration index after conflict
// resolution. StagePaths are added first; the final cached diff must be a
// subset of AllowedPaths.
func (a *taskMultiWorktreeAllocator) CommitStaged(
	ctx context.Context,
	spec runparallel.StagedCommitSpec,
) (string, error) {
	run, err := a.requireGitRunner()
	if err != nil {
		return "", err
	}
	worktreePath, err := requireTaskMultiWorktreeValue(spec.Path, "integration worktree path")
	if err != nil {
		return "", err
	}
	commitMessage, err := requireTaskMultiWorktreeValue(spec.Message, "integration commit message")
	if err != nil {
		return "", err
	}
	allowed, err := taskMultiNormalizeGitPaths(spec.AllowedPaths)
	if err != nil {
		return "", fmt.Errorf("validate staged commit allowed paths: %w", err)
	}
	stagePaths, err := taskMultiNormalizeGitPaths(spec.StagePaths)
	if err != nil {
		return "", fmt.Errorf("validate staged commit paths: %w", err)
	}
	return a.commitExplicitPaths(ctx, run, worktreePath, commitMessage, allowed, stagePaths)
}

func (a *taskMultiWorktreeAllocator) commitExplicitPaths(
	ctx context.Context,
	run taskMultiWorktreeGitRunner,
	worktreePath string,
	commitMessage string,
	allowedPaths []string,
	stagePaths []string,
) (string, error) {
	status, err := run(ctx, worktreePath, "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("inspect worktree status in %s: %w", worktreePath, err)
	}
	if conflicts := taskMultiWorktreeUnmergedFiles(status); len(conflicts) > 0 && len(stagePaths) == 0 {
		return "", fmt.Errorf(
			"worktree %s has unresolved merge conflicts: %s",
			worktreePath,
			strings.Join(conflicts, ", "),
		)
	}
	if len(stagePaths) > 0 {
		args := append([]string{"add", "-A", "--"}, stagePaths...)
		if _, err := run(ctx, worktreePath, args...); err != nil {
			return "", fmt.Errorf("stage explicit changes in %s: %w", worktreePath, err)
		}
	}
	status, err = run(ctx, worktreePath, "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("inspect staged changes in %s: %w", worktreePath, err)
	}
	if conflicts := taskMultiWorktreeUnmergedFiles(status); len(conflicts) > 0 {
		return "", fmt.Errorf(
			"worktree %s has unresolved merge conflicts: %s",
			worktreePath,
			strings.Join(conflicts, ", "),
		)
	}
	cachedPaths, err := taskMultiCachedDiffPaths(ctx, run, worktreePath)
	if err != nil {
		return "", err
	}
	if len(cachedPaths) == 0 {
		return a.worktreeHead(ctx, worktreePath)
	}
	if err := taskMultiValidateStagedSubset(cachedPaths, allowedPaths); err != nil {
		return "", fmt.Errorf("validate staged changes in %s: %w", worktreePath, err)
	}
	if _, err := run(ctx, worktreePath, "commit", "--no-verify", "-m", commitMessage); err != nil {
		return "", fmt.Errorf("commit explicit changes in %s: %w", worktreePath, err)
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
			staged, stagedErr := taskMultiCachedDiffPaths(ctx, run, path)
			if stagedErr != nil {
				return ConflictSet{}, stagedErr
			}
			return ConflictSet{Files: conflicts, StagedFiles: staged, Clean: false}, nil
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
			staged, stagedErr := taskMultiCachedDiffPaths(ctx, run, path)
			if stagedErr != nil {
				return ConflictSet{}, stagedErr
			}
			return ConflictSet{Files: conflicts, StagedFiles: staged, Clean: false}, nil
		}
		return ConflictSet{}, fmt.Errorf("squash merge %s into %s: %w", ref, path, err)
	}
	if _, err := run(ctx, path, "commit", "--allow-empty", "--no-verify", "-m", commitMessage); err != nil {
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

// DiscardIntegrationBranchIfExists removes an integration worktree and branch
// during explicit purge. Missing branches are acceptable because rollback may
// already have discarded the integration branch while preserving task worktrees.
func (a *taskMultiWorktreeAllocator) DiscardIntegrationBranchIfExists(
	ctx context.Context,
	workspaceRoot string,
	worktreesRoot string,
	integrationPath string,
	integrationBranch string,
) (string, bool, error) {
	run, err := a.requireGitRunner()
	if err != nil {
		return "", false, err
	}
	workspace, err := requireTaskMultiWorktreeValue(workspaceRoot, "workspace root")
	if err != nil {
		return "", false, err
	}
	path, err := requireTaskMultiWorktreeValue(integrationPath, "integration path")
	if err != nil {
		return "", false, err
	}
	branch, err := requireTaskMultiWorktreeValue(integrationBranch, "integration branch")
	if err != nil {
		return "", false, err
	}
	path, err = a.resolveIntegrationPurgePath(ctx, workspace, worktreesRoot, path, branch)
	if err != nil {
		return "", false, err
	}
	pathRemoved, err := a.removeIntegrationWorktreeForPurge(ctx, run, workspace, path)
	if err != nil {
		return "", false, err
	}
	if err := a.deleteIntegrationBranchIfExists(ctx, run, workspace, branch); err != nil {
		return path, pathRemoved, err
	}
	return path, pathRemoved, nil
}

func (a *taskMultiWorktreeAllocator) resolveIntegrationPurgePath(
	ctx context.Context,
	workspaceRoot string,
	worktreesRoot string,
	plannedPath string,
	branch string,
) (string, error) {
	registeredPath, err := a.integrationWorktreePathForBranch(ctx, workspaceRoot, branch)
	if err != nil {
		return "", err
	}
	if registeredPath != "" {
		ownedPath, ok, err := cleanOwnedWorktreePath(worktreesRoot, registeredPath)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf(
				"integration branch %s is checked out outside worktree root at %s",
				branch,
				registeredPath,
			)
		}
		return ownedPath, nil
	}
	ownedPath, ok, err := cleanOwnedWorktreePath(worktreesRoot, plannedPath)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf(
			"integration purge path is outside worktree root at %s",
			plannedPath,
		)
	}
	return ownedPath, nil
}

func (a *taskMultiWorktreeAllocator) removeIntegrationWorktreeForPurge(
	ctx context.Context,
	run taskMultiWorktreeGitRunner,
	workspaceRoot string,
	path string,
) (bool, error) {
	pathRemoved := false
	if _, statErr := os.Stat(path); statErr == nil {
		// Purge follows rollback semantics for integration worktrees: the branch
		// is internal scratch state and may contain conflict index state.
		if _, err := run(ctx, workspaceRoot, "worktree", "remove", "--force", path); err != nil {
			return false, fmt.Errorf("remove integration worktree %s: %w", path, err)
		}
		if _, statErr := os.Stat(path); statErr == nil {
			return false, fmt.Errorf("remove integration worktree %s: path still exists", path)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return false, fmt.Errorf("stat integration worktree %s after removal: %w", path, statErr)
		}
		pathRemoved = true
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return false, fmt.Errorf("stat integration worktree %s: %w", path, statErr)
	}
	return pathRemoved, nil
}

func (a *taskMultiWorktreeAllocator) deleteIntegrationBranchIfExists(
	ctx context.Context,
	run taskMultiWorktreeGitRunner,
	workspaceRoot string,
	branch string,
) error {
	branchExists, err := a.integrationBranchExists(ctx, workspaceRoot, branch)
	if err != nil {
		return err
	}
	if branchExists {
		if _, err := run(ctx, workspaceRoot, "branch", "-D", branch); err != nil {
			return fmt.Errorf("delete integration branch %s: %w", branch, err)
		}
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
	// Serialize teardown against concurrent allocations on the same repository.
	unlock := a.lockWorktreeMeta(workspace)
	defer unlock()
	if _, err := run(ctx, workspace, "worktree", "remove", worktreePath); err != nil {
		return fmt.Errorf("remove worktree %s: %w", worktreePath, err)
	}
	return nil
}

func (a *taskMultiWorktreeAllocator) DeleteBranchIfAt(
	ctx context.Context,
	workspaceRoot string,
	branch string,
	expectedCommit string,
) (bool, error) {
	run, err := a.requireGitRunner()
	if err != nil {
		return false, err
	}
	workspace, err := requireTaskMultiWorktreeValue(workspaceRoot, "workspace root")
	if err != nil {
		return false, err
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false, nil
	}
	expectedCommit = strings.TrimSpace(expectedCommit)
	if expectedCommit == "" {
		return false, errors.New("daemon: expected branch commit is required")
	}
	commit, err := run(ctx, workspace, "branch", "--list", branch, "--format=%(objectname)")
	if err != nil {
		return false, fmt.Errorf("inspect result branch %s: %w", branch, err)
	}
	commit = strings.TrimSpace(commit)
	if commit == "" || commit != expectedCommit {
		return false, nil
	}
	if _, err := run(ctx, workspace, "branch", "-d", branch); err != nil {
		return false, fmt.Errorf("delete empty result branch %s: %w", branch, err)
	}
	return true, nil
}

func (a *taskMultiWorktreeAllocator) integrationBranchExists(
	ctx context.Context,
	workspaceRoot string,
	branch string,
) (bool, error) {
	run, err := a.requireGitRunner()
	if err != nil {
		return false, err
	}
	out, err := run(ctx, workspaceRoot, "branch", "--list", branch, "--format=%(refname:short)")
	if err != nil {
		return false, fmt.Errorf("inspect integration branch %s: %w", branch, err)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == branch {
			return true, nil
		}
	}
	return false, nil
}

func (a *taskMultiWorktreeAllocator) integrationWorktreePathForBranch(
	ctx context.Context,
	workspaceRoot string,
	branch string,
) (string, error) {
	run, err := a.requireGitRunner()
	if err != nil {
		return "", err
	}
	out, err := run(ctx, workspaceRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("inspect integration worktrees for branch %s: %w", branch, err)
	}
	wantRef := "refs/heads/" + branch
	currentPath := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			currentPath = ""
		case strings.HasPrefix(line, "worktree "):
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		case strings.HasPrefix(line, "branch "):
			if strings.TrimSpace(strings.TrimPrefix(line, "branch ")) == wantRef {
				return currentPath, nil
			}
		}
	}
	return "", nil
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

func taskMultiResultBranch(parentRunID string, index int, slug string) (string, error) {
	if index < 0 {
		return "", fmt.Errorf("daemon: result branch index must be non-negative, got %d", index)
	}
	parent := sanitizeTaskMultiWorktreeSegment(parentRunID, taskMultiWorktreeParentShortLen)
	if parent == "" {
		return "", errors.New("daemon: result branch parent run id is required")
	}
	cleanSlug := sanitizeTaskMultiWorktreeSegment(slug, taskMultiWorktreeSlugMaxLen)
	if cleanSlug == "" {
		return "", fmt.Errorf("daemon: result branch slug %q is invalid", slug)
	}
	return fmt.Sprintf(
		"compozy/multi-%s-%s-%0*d-%s",
		parent,
		taskMultiShortHash(strings.TrimSpace(parentRunID), taskMultiWorktreeParentHashLen),
		taskMultiWorktreeIndexPadWidth,
		index,
		cleanSlug,
	), nil
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

func taskMultiCachedDiffPaths(
	ctx context.Context,
	run taskMultiWorktreeGitRunner,
	worktreePath string,
) ([]string, error) {
	raw, err := run(ctx, worktreePath, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return nil, fmt.Errorf("inspect cached diff in %s: %w", worktreePath, err)
	}
	return taskMultiParseNULPaths(raw), nil
}

func taskMultiParseNULPaths(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "\x00")
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		paths = append(paths, part)
	}
	return taskMultiUniqueSorted(paths)
}

func taskMultiNormalizeGitPaths(paths []string) ([]string, error) {
	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		value := strings.TrimSpace(path)
		if value == "" {
			continue
		}
		if filepath.IsAbs(value) {
			return nil, fmt.Errorf("absolute path %q is not allowed", value)
		}
		clean := filepath.Clean(filepath.FromSlash(value))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return nil, fmt.Errorf("path %q escapes worktree", value)
		}
		normalized = append(normalized, filepath.ToSlash(clean))
	}
	return taskMultiUniqueSorted(normalized), nil
}

func taskMultiValidateStagedSubset(stagedPaths []string, allowedPaths []string) error {
	allowed := make(map[string]struct{}, len(allowedPaths))
	for _, path := range allowedPaths {
		allowed[path] = struct{}{}
	}
	unexpected := make([]string, 0)
	for _, path := range stagedPaths {
		if _, ok := allowed[path]; !ok {
			unexpected = append(unexpected, path)
		}
	}
	if len(unexpected) > 0 {
		return fmt.Errorf("unexpected staged paths: %s", strings.Join(unexpected, ", "))
	}
	return nil
}

func taskMultiUniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	result := values[:0]
	var previous string
	for idx, value := range values {
		if idx > 0 && value == previous {
			continue
		}
		result = append(result, value)
		previous = value
	}
	return result
}

func runTaskMultiWorktreeGitCommand(ctx context.Context, dir string, args ...string) (string, error) {
	return gitenv.Run(ctx, dir, args...)
}
