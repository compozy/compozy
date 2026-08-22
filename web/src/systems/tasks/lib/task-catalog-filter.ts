import type { TaskInboxFilter, TaskListFilter } from "../types";

import type { TasksRouteSearch } from "./task-location-search";
import { taskOwnerFilterFromValue } from "./tasks-list-filters";
import type { ActiveTaskScopeFilter } from "./workspace-scope";

export const DEFAULT_TASK_LIST_LIMIT = 50;

export function defaultTaskCatalogFilter(scope: ActiveTaskScopeFilter): TaskListFilter {
  return {
    scope: scope.scope,
    workspace: scope.workspace,
    worktree: scope.worktree,
    include_drafts: true,
    limit: DEFAULT_TASK_LIST_LIMIT,
    sort: "recent",
  };
}

export interface TaskListRevealOptions {
  /**
   * Reveal filter state. Ephemeral per navigation (US-002.AC-3), so it is never
   * part of `TasksRouteSearch` and never reaches the URL.
   */
  includeLoop?: boolean;
}

export function taskListFilterFromRouteSearch(
  scope: ActiveTaskScopeFilter,
  search: TasksRouteSearch,
  reveal: TaskListRevealOptions = {}
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
    // Only `true` is ever sent. The daemon owns the default, so the calm read is
    // a request with no `include_loop` at all — not an explicit `false`.
    include_loop: reveal.includeLoop ? true : undefined,
  };
}

export function taskInboxFilterFromRouteSearch(
  scope: ActiveTaskScopeFilter,
  search: TasksRouteSearch
): TaskInboxFilter {
  return {
    scope: scope.scope,
    workspace: scope.workspace,
    worktree: scope.worktree,
    lane:
      search.inboxLane === undefined || search.inboxLane === "all" ? undefined : search.inboxLane,
    status: search.inboxStatus,
    priority: search.inboxPriority,
    unread: search.inboxUnread,
    query: search.inboxQuery?.trim() || undefined,
    limit: DEFAULT_TASK_LIST_LIMIT,
  };
}
