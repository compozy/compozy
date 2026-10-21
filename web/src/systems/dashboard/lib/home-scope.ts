import { type ActiveTaskScopeFilter, taskScopeForActiveWorkspace } from "@/systems/tasks";
import { workspaceSelectorOrAll, type DataScope } from "@/systems/workspace";

export interface HomeScope {
  /** The data scope Home reads; the label and every query derive from it. */
  dataScope: DataScope;
  /**
   * Workspace selector for dashboard queries: a project id, `@user` for
   * User, or empty for All projects (whole-system aggregates).
   */
  workspaceParam: string;
  taskScope: ActiveTaskScopeFilter;
}

/**
 * Maps the menubar destination onto dashboard query params. User reads
 * `@user`, never the aggregate. Returns `null` until the destination is known.
 */
export function homeScopeForActiveWorkspace(dataScope: DataScope | null): HomeScope | null {
  const taskScope = taskScopeForActiveWorkspace(dataScope);
  if (!dataScope || !taskScope) {
    return null;
  }
  return { dataScope, workspaceParam: workspaceSelectorOrAll(dataScope) ?? "", taskScope };
}
