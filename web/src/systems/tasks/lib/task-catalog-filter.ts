import type { TaskInboxFilter, TaskListFilter } from "../types";

import type { TasksRouteSearch } from "./task-location-search";
import { taskOwnerFilterFromValue } from "./tasks-list-filters";
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

export function taskListFilterFromRouteSearch(
  scope: ActiveTaskScopeFilter,
  search: TasksRouteSearch
): TaskListFilter {
  const owner = taskOwnerFilterFromValue(search.owner);
  return {
    ...defaultTaskCatalogFilter(scope),
    status: search.status,
    priority: search.priority,
    owner_kind: owner?.kind,
    owner_ref: owner?.ref,
    query: search.query?.trim() || undefined,
    sort: search.sort ?? "recent",
  };
}

export function taskInboxFilterFromRouteSearch(
  scope: ActiveTaskScopeFilter,
  search: TasksRouteSearch
): TaskInboxFilter {
  return {
    scope: scope.scope,
    workspace: scope.workspace,
    lane:
      search.inboxLane === undefined || search.inboxLane === "all" ? undefined : search.inboxLane,
    status: search.inboxStatus,
    priority: search.inboxPriority,
    unread: search.inboxUnread,
    query: search.inboxQuery?.trim() || undefined,
    limit: DEFAULT_TASK_LIST_LIMIT,
  };
}
