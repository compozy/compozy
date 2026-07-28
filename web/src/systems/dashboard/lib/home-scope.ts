import { taskScopeForActiveWorkspace, type ActiveTaskScopeFilter } from "@/systems/tasks";

type ActiveWorkspaceScopeCandidate = Parameters<typeof taskScopeForActiveWorkspace>[0];

export interface HomeScope {
  /** Empty selects the global home scope (whole-system aggregates). */
  workspaceParam: string;
  taskScope: ActiveTaskScopeFilter;
}

/**
 * Maps the home workspace to global scope and project workspaces to their id.
 * Returns `null` until both the active workspace and daemon home are known.
 */
export function homeScopeForActiveWorkspace(
  activeWorkspace: ActiveWorkspaceScopeCandidate | null | undefined,
  userHomeDir: string | undefined
): HomeScope | null {
  const scope = taskScopeForActiveWorkspace(activeWorkspace, userHomeDir);
  if (!scope) {
    return null;
  }
  if (scope.scope === "global") {
    return { workspaceParam: "", taskScope: { scope: "global" } };
  }
  return { workspaceParam: scope.workspace, taskScope: scope };
}
