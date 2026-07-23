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

	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/gitenv"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/store/globaldb"
)

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
		canonicalRoot = active.workspaceRoot
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
	for _, taskGroupID := range completed {
		m.syncHydratedTaskGroupPlanBestEffort(
			ctx,
			workspaceRoot,
			initiative+"/"+taskGroupID,
			runID,
		)
	}
	if len(marked) > 0 {
		slog.Default().Info(
			"daemon: hydrated task group completion after run",
			"run_id", runID,
			"workspace_root", workspaceRoot,
			"initiative", initiative,
			"marked", marked,
		)
	}
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
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		path := filepath.Clean(strings.TrimSpace(strings.TrimPrefix(line, "worktree ")))
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
