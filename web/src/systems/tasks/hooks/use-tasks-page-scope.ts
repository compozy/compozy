import { useActiveWorkspace, useActiveWorktree, useWorktrees } from "@/systems/workspace";
import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { useProfileReadScope } from "@/systems/profiles";

import { taskScopeForActiveWorkspace, type ActiveTaskScopeFilter } from "../lib/workspace-scope";

interface TaskScopeStatus {
  hasActiveTaskScope: boolean;
  scopeError: Error | null;
  scopeLoading: boolean;
}

function resolveTaskScopeError(
  hasActiveTaskScope: boolean,
  scopeLoading: boolean,
  scopeSourceError: Error | null
): Error | null {
  if (hasActiveTaskScope || scopeLoading) return null;
  if (scopeSourceError) return scopeSourceError;
  return new Error("No active workspace is available for task scope.");
}

/** A scope still resolving is loading; one that cannot resolve names why. */
function resolveTaskScopeStatus(
  activeTaskScope: ActiveTaskScopeFilter | null,
  worktreeScopeResolved: boolean,
  workspace: ReturnType<typeof useActiveWorkspace>
): TaskScopeStatus {
  const hasActiveTaskScope = activeTaskScope !== null;
  const scopeSourceError =
    workspace.scope === "workspace" && !workspace.activeWorkspaceId
      ? (workspace.error ?? null)
      : null;
  const scopeLoading =
    !hasActiveTaskScope &&
    !scopeSourceError &&
    (!worktreeScopeResolved || !workspace.hasHydrated || workspace.pending);
  return {
    hasActiveTaskScope,
    scopeError: resolveTaskScopeError(hasActiveTaskScope, scopeLoading, scopeSourceError),
    scopeLoading,
  };
}

/**
 * The task listing's scope: this window's workspace and worktree. Filtering
 * happens server-side on the derived active-run worktree id — a loaded page is
 * never trimmed client-side. Global cannot bind a worktree; the scope helper
 * drops it on that branch. The profile axis rides along: owner tags under the
 * aggregate, and the profile the empty state names.
 */
function useTasksPageScope(liveDataEnabled: boolean) {
  const workspace = useActiveWorkspace({ enabled: liveDataEnabled });
  const { activeWorkspaceId, scope } = workspace;
  const profile = useProfileReadScope();
  const worktreeScopeId = useWorktreeScopeId();
  const worktreesQuery = useWorktrees(activeWorkspaceId, {
    enabled: liveDataEnabled && activeWorkspaceId !== null,
  });
  const worktreeSelection = useActiveWorktree(worktreeScopeId, worktreesQuery.data);
  const worktreeScopeResolved = scope !== "workspace" || worktreeSelection.resolved;
  const activeTaskScope = worktreeScopeResolved
    ? taskScopeForActiveWorkspace(
        scope,
        activeWorkspaceId,
        worktreeSelection.activeWorktree?.id ?? null
      )
    : null;

  return {
    ...resolveTaskScopeStatus(activeTaskScope, worktreeScopeResolved, workspace),
    activeTaskScope,
    activeWorkspaceName: workspace.activeWorkspace?.name ?? null,
    profile,
  };
}

export { useTasksPageScope };
export type { TaskScopeStatus };
