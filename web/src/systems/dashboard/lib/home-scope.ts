import { type ActiveTaskScopeFilter, taskScopeForActiveWorkspace } from "@/systems/tasks";

export interface HomeScope {
  /** Empty selects the global home scope (whole-system aggregates). */
  workspaceParam: string;
  taskScope: ActiveTaskScopeFilter;
}

/**
 * Maps menubar-owned scope onto dashboard query params.
 * Returns `null` until the destination is known.
 */
export function homeScopeForActiveWorkspace(
  scope: "global" | "workspace" | null | undefined,
  workspaceId?: string | null
): HomeScope | null {
  const taskScope = taskScopeForActiveWorkspace(scope, workspaceId);
  if (!taskScope) {
    return null;
  }
  if (taskScope.scope === "global") {
    return { workspaceParam: "", taskScope: { scope: "global" } };
  }
  return { workspaceParam: taskScope.workspace, taskScope };
}
