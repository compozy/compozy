import type { TaskListFilter } from "../types";

import type { ActiveTaskScopeFilter } from "./workspace-scope";

export const DEFAULT_TASK_LIST_LIMIT = 50;

export function defaultTaskCatalogFilter(scope: ActiveTaskScopeFilter): TaskListFilter {
  return {
    scope: scope.scope,
    workspace: scope.workspace,
    include_drafts: true,
    limit: DEFAULT_TASK_LIST_LIMIT,
    sort: "recent",
  };
}
