import { type ActiveTaskScopeFilter, taskScopeForActiveWorkspace } from "@/systems/tasks";
import { workspaceSelectorOrAll, type DataScope } from "@/systems/workspace";

export interface HomeScope {
  /** The data scope Home reads; the label and every query derive from it. */
  dataScope: DataScope;
  /**
   * Workspace selector for per-scope reads (sessions, activity, attention): a
   * project id, `@user` for User, or empty for All projects, whose omission
   * already reads every project.
   */
  workspaceParam: string;
  /** Task-route breadth: `all_workspaces` under All projects, User's unbound scope, or a project. */
  taskScope: ActiveTaskScopeFilter;
  /**
   * Overview breadth. Its omission reads only User's tasks, so All projects
   * asks for the aggregate explicitly.
   */
  overview: { allWorkspaces: true } | { workspace: string };
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
  const selector = workspaceSelectorOrAll(dataScope);
  return {
    dataScope,
    workspaceParam: selector ?? "",
    taskScope,
    overview: selector === null ? { allWorkspaces: true } : { workspace: selector },
  };
}
