import {
  breadthQuery,
  taskBreadthParams,
  type DataScope,
  type ProjectFilterValue,
} from "@/systems/workspace";

/**
 * Task-route breadth for a data scope: the route's encoder (`all_workspaces`
 * plus an optional `workspace_ids` narrowing under All projects, User's
 * unbound scope, or one project), plus a worktree. Only a project carries a
 * worktree — the daemon rejects one without a workspace.
 */
export type ActiveTaskScopeFilter = ReturnType<typeof taskBreadthParams> & { worktree?: string };

/**
 * Task listing breadth for a data scope and its Project filter (All projects
 * only). `null` while the destination is unsettled.
 */
export function taskScopeForActiveWorkspace(
  dataScope: DataScope | null | undefined,
  activeWorktreeId?: string | null,
  projectFilter?: readonly ProjectFilterValue[]
): ActiveTaskScopeFilter | null {
  if (!dataScope) return null;
  const params = taskBreadthParams(breadthQuery(dataScope, projectFilter));
  const worktree = dataScope.kind === "project" ? activeWorktreeId?.trim() : undefined;
  return worktree ? { ...params, worktree } : params;
}

/**
 * Omitting the inbox scope reads every task, which is exactly All projects, so
 * the aggregate is sent only to carry its Project filter (`workspace_ids`).
 */
export function taskInboxScope(scope: ActiveTaskScopeFilter | null) {
  if (scope?.all_workspaces && scope.workspace_ids?.length) {
    return { all_workspaces: true, workspace_ids: scope.workspace_ids };
  }
  return { scope: scope?.scope, workspace: scope?.workspace, worktree: scope?.worktree };
}

/** The scheduler backlog spells the aggregate `scope=all`; it has no Project filter. */
export function schedulerBacklogScope(scope: ActiveTaskScopeFilter): {
  scope?: "all" | "global" | "workspace";
  workspace?: string;
} {
  return scope.all_workspaces
    ? { scope: "all" }
    : { scope: scope.scope, workspace: scope.workspace };
}
