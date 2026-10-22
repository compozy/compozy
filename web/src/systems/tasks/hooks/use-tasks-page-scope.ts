import {
  useActiveWorkspace,
  useActiveWorktree,
  useProjectFilter,
  useWindowDataWorkspace,
  useWorktrees,
  workspaceBadgeIdentity,
  type WorkspaceBadgeIdentity,
} from "@/systems/workspace";
import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { useProfileReadScope } from "@/systems/profiles";

import { taskScopeForActiveWorkspace, type ActiveTaskScopeFilter } from "../lib/workspace-scope";
import type { TaskListItem } from "../types";

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
  const scopeSourceError = workspace.dataScope === null ? (workspace.error ?? null) : null;
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
 * The task listing's scope: this window's data scope (its binding, else its
 * partition — the aggregate on the All projects desktop) and, for a project,
 * its worktree. Filtering happens server-side on the derived active-run
 * worktree id — a loaded page is never trimmed client-side. Under All projects
 * the list (and kanban) also narrow by the transient Project filter; the
 * dashboard, inbox, and badge read the whole breadth. The profile axis rides
 * along: owner tags under the aggregate, and the profile the empty state names.
 */
function useTasksPageScope(liveDataEnabled: boolean) {
  const workspace = useActiveWorkspace({ enabled: liveDataEnabled });
  const dataScope = useWindowDataWorkspace();
  const projectId = dataScope?.kind === "project" ? dataScope.workspaceId : null;
  const profile = useProfileReadScope();
  const projectFilter = useProjectFilter(dataScope, workspace.workspaces);
  const worktreeScopeId = useWorktreeScopeId();
  const worktreesQuery = useWorktrees(projectId, {
    enabled: liveDataEnabled && projectId !== null,
  });
  const worktreeSelection = useActiveWorktree(worktreeScopeId, worktreesQuery.data);
  const worktreeScopeResolved = projectId === null || worktreeSelection.resolved;
  const worktreeId = worktreeSelection.activeWorktree?.id ?? null;
  const activeTaskScope = worktreeScopeResolved
    ? taskScopeForActiveWorkspace(dataScope, worktreeId)
    : null;
  const listTaskScope = worktreeScopeResolved
    ? taskScopeForActiveWorkspace(dataScope, worktreeId, projectFilter.values)
    : null;

  const names = new Map(workspace.registeredWorkspaces.map(entry => [entry.id, entry.name]));

  return {
    ...resolveTaskScopeStatus(activeTaskScope, worktreeScopeResolved, workspace),
    activeTaskScope,
    listTaskScope,
    activeWorkspaceName: projectId === null ? null : (names.get(projectId) ?? null),
    profile,
    projectFilter,
    /** Each row's project under All projects (a mixed list); `undefined` when one scope is shown. */
    projectBadgeOf:
      dataScope?.kind === "all"
        ? (task: Pick<TaskListItem, "workspace_id">): WorkspaceBadgeIdentity =>
            workspaceBadgeIdentity(task.workspace_id, names)
        : undefined,
  };
}

export { useTasksPageScope };
export type { TaskScopeStatus };
