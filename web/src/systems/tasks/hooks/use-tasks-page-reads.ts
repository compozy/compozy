import { useTaskInbox, useTaskInboxBadge } from "./use-task-inbox";
import { useTasks } from "./use-tasks";
import type { TaskScopeStatus } from "./use-tasks-page-scope";
import type { TaskInboxFilter, TaskListFilter, TaskViewMode } from "../types";
import { groupTasksForKanban } from "@/systems/tasks/lib/task-grouping";
import type { KanbanColumnGroup } from "@/systems/tasks/lib/task-grouping";
import { taskStatusCountsFromFacets } from "@/systems/tasks/lib/task-list-query";

import {
  taskInboxFilterFromRouteSearch,
  taskListFilterFromRouteSearch,
} from "../lib/task-catalog-filter";
import type { TasksRouteSearch } from "../lib/task-location-search";
import type { ActiveTaskScopeFilter } from "../lib/workspace-scope";

type PagedQuery = {
  isFetchNextPageError: boolean;
  fetchNextPage: () => Promise<unknown>;
  refetch: () => Promise<unknown>;
};

/** A failed next page retries that page; anything else refetches from the top. */
function retryPagedQuery(query: PagedQuery) {
  if (query.isFetchNextPageError) {
    void query.fetchNextPage();
    return;
  }
  void query.refetch();
}

interface TasksPageListInput {
  activeTaskScope: ActiveTaskScopeFilter | null;
  enabled: boolean;
  hasListFilters: boolean;
  includeLoop: boolean;
  mode: TaskViewMode;
  routeSearch: TasksRouteSearch;
  routeSearchQuery: string;
  scopeStatus: TaskScopeStatus;
}

function taskListFilters({
  activeTaskScope,
  includeLoop,
  routeSearch,
  routeSearchQuery,
}: TasksPageListInput): TaskListFilter {
  if (!activeTaskScope) return {};
  return taskListFilterFromRouteSearch(
    activeTaskScope,
    { ...routeSearch, query: routeSearchQuery || undefined },
    { includeLoop }
  );
}

function taskListReadModel(tasksQuery: ReturnType<typeof useTasks>, input: TasksPageListInput) {
  const { scopeError, scopeLoading } = input.scopeStatus;
  const visibleTasks = tasksQuery.data ?? [];
  // The server already cut the population, so the board renders exactly what the
  // list does — counts stay coherent by construction (US-001.AC-3).
  const kanbanColumns: KanbanColumnGroup[] =
    input.mode === "kanban" ? groupTasksForKanban(visibleTasks) : [];
  // `isPending`, not `isLoading`: a suspended window disables the read, and a
  // disabled read with no data yet is still waiting, not an empty project.
  const isEmpty =
    input.scopeStatus.hasActiveTaskScope &&
    !scopeLoading &&
    !scopeError &&
    !tasksQuery.isPending &&
    !tasksQuery.error &&
    tasksQuery.total === 0 &&
    !input.hasListFilters;
  return {
    effectiveSelectedTaskId: visibleTasks[0]?.id ?? null,
    hasMoreTasks: tasksQuery.hasNextPage,
    isEmpty,
    isLoadingMoreTasks: tasksQuery.isFetchingNextPage,
    kanbanColumns,
    listError: scopeError ?? tasksQuery.error ?? null,
    listLoading: scopeLoading || (tasksQuery.isPending && visibleTasks.length === 0),
    listUpdatedAt: tasksQuery.dataUpdatedAt,
    ownerOptions: tasksQuery.facets.owners.map(facet => ({
      kind: facet.owner.kind,
      ref: facet.owner.ref,
    })),
    statusCounts: taskStatusCountsFromFacets(tasksQuery.facets),
    tasksCount: tasksQuery.dataUpdatedAt > 0 ? tasksQuery.total : undefined,
    visibleTasks,
  };
}

/** The list and kanban read: one server-filtered page stream for both surfaces. */
function useTasksPageList(input: TasksPageListInput) {
  const tasksQuery = useTasks(taskListFilters(input), { enabled: input.enabled });
  return {
    ...taskListReadModel(tasksQuery, input),
    loadMoreTasks: () => {
      void tasksQuery.fetchNextPage();
    },
    retryTasks: () => retryPagedQuery(tasksQuery),
  };
}

interface TasksPageInboxInput {
  activeTaskScope: ActiveTaskScopeFilter | null;
  badgeEnabled: boolean;
  enabled: boolean;
  routeInboxSearchQuery: string;
  routeSearch: TasksRouteSearch;
  scopeStatus: TaskScopeStatus;
}

function taskInboxFilters({
  activeTaskScope,
  routeInboxSearchQuery,
  routeSearch,
}: TasksPageInboxInput): TaskInboxFilter {
  if (!activeTaskScope) return {};
  return taskInboxFilterFromRouteSearch(activeTaskScope, {
    ...routeSearch,
    inboxQuery: routeInboxSearchQuery || undefined,
  });
}

function taskInboxReadModel(
  inboxQuery: ReturnType<typeof useTaskInbox>,
  scopeStatus: TaskScopeStatus
) {
  return {
    hasMoreInbox: inboxQuery.hasNextPage,
    inbox: inboxQuery.data ?? null,
    inboxError: scopeStatus.scopeError ?? inboxQuery.error ?? null,
    inboxLoading: scopeStatus.scopeLoading || (inboxQuery.isPending && !inboxQuery.data),
    inboxUpdatedAt: inboxQuery.dataUpdatedAt,
    isLoadingMoreInbox: inboxQuery.isFetchingNextPage,
  };
}

/** The inbox surface read plus the always-on unread badge for the window's scope. */
function useTasksPageInbox(input: TasksPageInboxInput) {
  const { activeTaskScope } = input;
  const inboxQuery = useTaskInbox(taskInboxFilters(input), { enabled: input.enabled });
  const inboxBadgeQuery = useTaskInboxBadge(
    {
      scope: activeTaskScope?.scope,
      workspace: activeTaskScope?.workspace,
      worktree: activeTaskScope?.worktree,
      limit: 1,
    },
    { enabled: input.badgeEnabled }
  );
  return {
    ...taskInboxReadModel(inboxQuery, input.scopeStatus),
    inboxUnreadCount: inboxBadgeQuery.data?.unread_total,
    loadMoreInbox: () => {
      void inboxQuery.fetchNextPage();
    },
    retryInbox: () => retryPagedQuery(inboxQuery),
  };
}

export { useTasksPageInbox, useTasksPageList };
