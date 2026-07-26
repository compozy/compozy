package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/gitenv"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// taskGroupHydrationTimeout bounds the best-effort completion hydration that runs
// on the run-terminal path. finishRun hands this work a context.WithoutCancel
// context (no deadline, never canceled), and the downstream plan lock
// (taskgroups.Store.withPlanLock -> flock.TryLockContext) retries acquisition until
// its context completes. Without a deadline, a plan lock held by a sibling
// worktree's `task-groups complete` bridge would block finishRun forever. Bailing
// out on expiry is safe: hydration is an additive projection that self-heals on the
// next resolveTaskGroupPreflightEvidence read. Declared as a var so tests can
// shorten it; production never reassigns it.
var taskGroupHydrationTimeout = 10 * time.Second

func (m *RunManager) hydrateTaskGroupPlanBestEffort(
	ctx context.Context,
	workspaceRoot, initiative string,
) {
	if m == nil || m.hydratePlanCompletion == nil {
		return
	}
	marked, err := m.hydratePlanCompletion(ctx, workspaceRoot, initiative)
	if err != nil {
		slog.Default().Warn(
			"daemon: hydrate task group completion",
			"workspace_root", workspaceRoot,
			"initiative", initiative,
			"error", err,
		)
		return
	}
	if len(marked) > 0 {
		slog.Default().Info(
			"daemon: hydrated task group completion",
			"workspace_root", workspaceRoot,
			"initiative", initiative,
			"marked", marked,
		)
	}
}

func (m *RunManager) hydrateTaskGroupCompletionAfterRun(
	ctx context.Context,
	active *activeRun,
	row globaldb.Run,
) {
	if active == nil {
		return
	}
	// Bound the detached run-terminal context so a contended plan lock cannot wedge
	// finishRun; see taskGroupHydrationTimeout.
	ctx, cancel := context.WithTimeout(ctx, taskGroupHydrationTimeout)
	defer cancel()
	ref, err := taskgroups.ParseTaskGroupRef(active.workflowSlug)
	if err != nil {
		return
	}
	completed, err := corepkg.CompletedTaskGroupIDsWithDB(
		ctx,
		m.globalDB,
		active.workspaceRoot,
		ref.Initiative,
	)
	if err != nil {
		slog.Default().Warn(
			"daemon: read task group completion authority",
			"run_id", row.RunID,
			"workspace_root", active.workspaceRoot,
			"initiative", ref.Initiative,
			"error", err,
		)
		return
	}
	if len(completed) == 0 {
		return
	}
	canonicalRoot, err := m.taskGroupHydrationCanonicalRoot(ctx, active.workspaceRoot, row)
	if err != nil {
		slog.Default().Warn(
			"daemon: resolve canonical task group hydration root",
			"run_id", row.RunID,
			"initiative", ref.Initiative,
			"error", err,
		)
		// A group-parallel child's workspaceRoot is its ephemeral worktree, not the
		// repository. Falling back to it would leave taskGroupHydrationRoots targeting
		// only worktrees that cleanupSettledTaskWorktree is about to remove, so the
		// user's repository plan never records the completion. Resolve the canonical
		// repository root from the worktree's git common dir instead; skip the fan-out
		// entirely if even that fails, since hydration self-heals on the next preflight.
		canonicalRoot, err = repositoryRootFromWorktree(ctx, active.workspaceRoot)
		if err != nil {
			slog.Default().Warn(
				"daemon: resolve repository root for task group hydration",
				"run_id", row.RunID,
				"initiative", ref.Initiative,
				"error", err,
			)
			return
		}
	}
	roots, err := m.taskGroupHydrationRoots(ctx, canonicalRoot)
	if err != nil {
		slog.Default().Warn(
			"daemon: enumerate task group hydration worktrees",
			"run_id", row.RunID,
			"initiative", ref.Initiative,
			"error", err,
		)
		roots = []string{canonicalRoot}
	}
	for _, root := range roots {
		m.hydrateTaskGroupCompletionRoot(ctx, root, ref.Initiative, row.RunID, completed)
	}
}

func (m *RunManager) hydrateTaskGroupCompletionRoot(
	ctx context.Context,
	workspaceRoot, initiative, runID string,
	completed []string,
) {
	planPath := filepath.Join(
		model.TasksBaseDirForWorkspace(workspaceRoot),
		initiative,
		taskgroups.ManifestFileName,
	)
	if _, err := os.Stat(planPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		slog.Default().Warn(
			"daemon: inspect task group hydration plan",
			"run_id", runID,
			"plan_path", planPath,
			"error", err,
		)
		return
	}
	initiativeDir := filepath.Dir(planPath)
	marked, err := taskgroups.NewStore().HydrateCompletion(ctx, initiativeDir, completed)
	if err != nil {
		slog.Default().Warn(
			"daemon: hydrate task group completion after run",
			"run_id", runID,
			"workspace_root", workspaceRoot,
			"initiative", initiative,
			"error", err,
		)
		return
	}
	if len(marked) == 0 {
		return
	}
	for _, taskGroupID := range marked {
		if context.Cause(ctx) != nil {
			return
		}
		m.syncHydratedTaskGroupPlanBestEffort(
			ctx,
			workspaceRoot,
			initiative+"/"+taskGroupID,
			runID,
		)
	}
	slog.Default().Info(
		"daemon: hydrated task group completion after run",
		"run_id", runID,
		"workspace_root", workspaceRoot,
		"initiative", initiative,
		"marked", marked,
	)
}

func (m *RunManager) syncHydratedTaskGroupPlanBestEffort(
	ctx context.Context,
	workspaceRoot, reference, runID string,
) {
	target, err := (taskgroups.TargetResolver{}).ResolveTaskGroup(ctx, workspaceRoot, reference)
	if err != nil {
		slog.Default().Warn(
			"daemon: resolve hydrated task group for catalog sync",
			"run_id", runID,
			"workspace_root", workspaceRoot,
			"reference", reference,
			"error", err,
		)
		return
	}
	scope, err := taskgroups.BuildExecutionScope(target)
	if err != nil {
		slog.Default().Warn(
			"daemon: build hydrated task group sync scope",
			"run_id", runID,
			"workspace_root", workspaceRoot,
			"reference", reference,
			"error", err,
		)
		return
	}
	workspace, err := m.globalDB.ResolveOrRegister(ctx, workspaceRoot)
	if err != nil {
		slog.Default().Warn(
			"daemon: resolve hydrated task group sync workspace",
			"run_id", runID,
			"workspace_root", workspaceRoot,
			"reference", reference,
			"error", err,
		)
		return
	}
	if _, err := m.syncWorkflow(
		ctx,
		m.globalDB,
		workspace,
		model.SyncConfig{ExecutionScope: &scope},
	); err != nil {
		slog.Default().Warn(
			"daemon: sync hydrated task group completion",
			"run_id", runID,
			"workspace_root", workspaceRoot,
			"reference", reference,
			"error", err,
		)
	}
}

func (m *RunManager) taskGroupHydrationCanonicalRoot(
	ctx context.Context,
	fallbackRoot string,
	row globaldb.Run,
) (string, error) {
	if m == nil || m.globalDB == nil {
		return "", errors.New("daemon: global database is required for completion hydration")
	}
	current := row
	visited := make(map[string]struct{})
	for strings.TrimSpace(current.ParentRunID) != "" {
		parentRunID := strings.TrimSpace(current.ParentRunID)
		if _, exists := visited[parentRunID]; exists {
			return "", fmt.Errorf("daemon: cyclic parent run chain at %s", parentRunID)
		}
		visited[parentRunID] = struct{}{}
		parent, err := m.globalDB.GetRun(ctx, parentRunID)
		if err != nil {
			return "", fmt.Errorf("load parent run %s: %w", parentRunID, err)
		}
		current = parent
	}
	if strings.TrimSpace(current.WorkspaceID) == "" {
		return filepath.Clean(fallbackRoot), nil
	}
	workspace, err := m.globalDB.Get(ctx, current.WorkspaceID)
	if err != nil {
		return "", fmt.Errorf("load parent run workspace: %w", err)
	}
	return workspace.RootDir, nil
}

// repositoryRootFromWorktree resolves the canonical repository root that owns the
// given worktree via git's common directory. Compozy runs group-parallel children
// inside ephemeral worktrees under the home worktrees dir; completion hydration must
// project into the user's real repository, whose root is the parent of the git
// common dir reported from any of its worktrees. The main worktree resolves to
// itself, so this is also correct for non-parallel runs.
func repositoryRootFromWorktree(ctx context.Context, worktreeRoot string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(worktreeRoot))
	if cleaned == "." || cleaned == "" {
		return "", errors.New("daemon: worktree root is required to resolve repository root")
	}
	output, err := gitenv.Run(ctx, cleaned, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("resolve repository git common dir: %w", err)
	}
	commonDir := filepath.Clean(strings.TrimSpace(output))
	if commonDir == "." || commonDir == "" {
		return "", errors.New("daemon: empty git common dir for repository root")
	}
	return filepath.Dir(commonDir), nil
}

func (m *RunManager) taskGroupHydrationRoots(
	ctx context.Context,
	canonicalRoot string,
) ([]string, error) {
	canonicalRoot = filepath.Clean(strings.TrimSpace(canonicalRoot))
	if canonicalRoot == "." || canonicalRoot == "" {
		return nil, errors.New("daemon: canonical hydration root is required")
	}
	output, err := gitenv.Run(ctx, canonicalRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list task group worktrees: %w", err)
	}
	seen := map[string]struct{}{canonicalRoot: {}}
	owned := make([]string, 0)
	// Route the porcelain parse through the shared gitenv primitive; the daemon
	// write fan-out keeps its home-owned ownership filter below (ADR-002, ADR-004).
	for _, path := range gitenv.ParseWorktreeList(output) {
		if path == canonicalRoot || !looksLikeCompozyWorktreePath(path) {
			continue
		}
		ownedPath, ok, ownershipErr := cleanOwnedWorktreePath(m.homePaths.WorktreesDir, path)
		if ownershipErr != nil {
			return nil, ownershipErr
		}
		if !ok {
			continue
		}
		if _, exists := seen[ownedPath]; exists {
			continue
		}
		seen[ownedPath] = struct{}{}
		owned = append(owned, ownedPath)
	}
	sort.Strings(owned)
	return append([]string{canonicalRoot}, owned...), nil
}
