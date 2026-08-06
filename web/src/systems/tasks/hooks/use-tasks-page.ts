import { useEffect, useRef, useState } from "react";

import { useTaskInbox, useTaskInboxBadge } from "./use-task-inbox";
import { useTasks } from "./use-tasks";
import type {
  TaskDashboardFilter,
  TaskInboxFilter,
  TaskListFilter,
  TaskListSortKey,
  TaskPriority,
  TaskStatus,
  TaskViewMode,
} from "../types";
import { groupTasksForKanban } from "@/systems/tasks/lib/task-grouping";
import type { KanbanColumnGroup } from "@/systems/tasks/lib/task-grouping";
import { buildTaskListTree } from "@/systems/tasks/lib/task-hierarchy";
import type { InboxLaneFilterId } from "@/systems/tasks/lib/inbox-grouping";
import { taskStatusCountsFromFacets } from "@/systems/tasks/lib/task-list-query";
import {
  taskOwnerFilterFromValue,
  taskOwnerFilterValue,
  type TaskFilterOwnerOption,
} from "@/systems/tasks/lib/tasks-list-filters";
import { useDaemonStatus } from "@/systems/status";
import { useActiveWorkspace } from "@/systems/workspace";

import {
  taskInboxFilterFromRouteSearch,
  taskListFilterFromRouteSearch,
} from "../lib/task-catalog-filter";
import {
  parseTasksSurfaceMode,
  validateTasksSearch,
  type TasksRouteSearch,
} from "../lib/task-location-search";
import { useTasksDashboardPage } from "./use-tasks-dashboard-page";
import { useTasksPageActions } from "./use-tasks-page-actions";
import { taskScopeForActiveWorkspace } from "../lib/workspace-scope";

type InboxLaneFilter = InboxLaneFilterId;
const SEARCH_DEBOUNCE_MS = 200;

interface UseTasksPageOptions {
  /** Validated URL state. The route is the sole owner of catalog filters. */
  search?: TasksRouteSearch;
  onSearchChange?: (update: (current: TasksRouteSearch) => TasksRouteSearch) => void;
  forceListData?: boolean;
}

function resolveTaskScopeError(
  hasActiveTaskScope: boolean,
  scopeLoading: boolean,
  scopeSourceError: Error | null,
  userHomeDir: string | undefined
): Error | null {
  if (hasActiveTaskScope || scopeLoading) return null;
  if (scopeSourceError) return scopeSourceError;
  if (!userHomeDir) return new Error("Daemon status did not provide a user home directory.");
  return new Error("No active workspace is available for task scope.");
}

function useTasksPage(options: UseTasksPageOptions = {}) {
  const workspace = useActiveWorkspace();
  const daemonStatus = useDaemonStatus();
  const { activeWorkspace } = workspace;
  const userHomeDir = daemonStatus.data?.user_home_dir;
  const routeSearch = options.search ?? {};
  const mode: TaskViewMode = parseTasksSurfaceMode(routeSearch);
  const statusFilter = routeSearch.status ?? null;
  const ownerFilter = taskOwnerFilterFromValue(routeSearch.owner);
  const priorityFilter = routeSearch.priority ?? null;
  const sortBy = routeSearch.sort ?? "recent";
  const routeSearchQuery = routeSearch.query ?? "";
  const inboxLaneFilter = routeSearch.inboxLane ?? "all";
  const inboxStatusFilter = routeSearch.inboxStatus ?? null;
  const inboxPriorityFilter = routeSearch.inboxPriority ?? null;
  const inboxUnreadOnly = routeSearch.inboxUnread === true;
  const routeInboxSearchQuery = routeSearch.inboxQuery ?? "";
  const [searchDraft, setSearchDraft] = useState({
    routeValue: routeSearchQuery,
    value: routeSearchQuery,
  });
  const [inboxSearchDraft, setInboxSearchDraft] = useState({
    routeValue: routeInboxSearchQuery,
    value: routeInboxSearchQuery,
  });
  const searchQuery =
    searchDraft.routeValue === routeSearchQuery ? searchDraft.value : routeSearchQuery;
  const inboxSearchQuery =
    inboxSearchDraft.routeValue === routeInboxSearchQuery
      ? inboxSearchDraft.value
      : routeInboxSearchQuery;
  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inboxSearchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const updateSearch = (patch: Partial<TasksRouteSearch>) => {
    options.onSearchChange?.(current =>
      validateTasksSearch({
        ...current,
        ...patch,
      })
    );
  };

  useEffect(() => {
    return () => {
      if (searchDebounceRef.current) clearTimeout(searchDebounceRef.current);
      if (inboxSearchDebounceRef.current) clearTimeout(inboxSearchDebounceRef.current);
    };
  }, []);

  useEffect(() => {
    if (searchDebounceRef.current) {
      clearTimeout(searchDebounceRef.current);
      searchDebounceRef.current = null;
    }
  }, [routeSearchQuery]);

  useEffect(() => {
    if (inboxSearchDebounceRef.current) {
      clearTimeout(inboxSearchDebounceRef.current);
      inboxSearchDebounceRef.current = null;
    }
  }, [routeInboxSearchQuery]);

  const setSearchQuery = (query: string) => {
    setSearchDraft({ routeValue: routeSearchQuery, value: query });
    if (searchDebounceRef.current) clearTimeout(searchDebounceRef.current);
    searchDebounceRef.current = setTimeout(() => {
      searchDebounceRef.current = null;
      updateSearch({ query: query.trim() ? query : undefined });
    }, SEARCH_DEBOUNCE_MS);
  };

  const setInboxSearchQuery = (query: string) => {
    setInboxSearchDraft({ routeValue: routeInboxSearchQuery, value: query });
    if (inboxSearchDebounceRef.current) clearTimeout(inboxSearchDebounceRef.current);
    inboxSearchDebounceRef.current = setTimeout(() => {
      inboxSearchDebounceRef.current = null;
      updateSearch({ inboxQuery: query.trim() ? query : undefined });
    }, SEARCH_DEBOUNCE_MS);
  };

  const activeTaskScope = taskScopeForActiveWorkspace(activeWorkspace, userHomeDir);
  const hasActiveTaskScope = activeTaskScope !== null;
  const scopeSourceError =
    (!userHomeDir ? daemonStatus.error : null) ??
    (!activeWorkspace ? workspace.error : null) ??
    null;
  const scopeLoading =
    !hasActiveTaskScope &&
    !scopeSourceError &&
    (!workspace.hasHydrated || workspace.isPending || daemonStatus.isPending);
  const scopeError = resolveTaskScopeError(
    hasActiveTaskScope,
    scopeLoading,
    scopeSourceError,
    userHomeDir
  );
  const listFilters: TaskListFilter = activeTaskScope
    ? taskListFilterFromRouteSearch(activeTaskScope, {
        ...routeSearch,
        query: routeSearchQuery || undefined,
      })
    : {};
  const dashboardFilters: TaskDashboardFilter = {
    scope: activeTaskScope?.scope,
    workspace: activeTaskScope?.workspace,
  };
  const inboxFilters: TaskInboxFilter = activeTaskScope
    ? taskInboxFilterFromRouteSearch(activeTaskScope, {
        ...routeSearch,
        inboxQuery: routeInboxSearchQuery || undefined,
      })
    : {};
  const inboxBadgeFilters: TaskInboxFilter = {
    scope: activeTaskScope?.scope,
    workspace: activeTaskScope?.workspace,
    limit: 1,
  };

  const isListTab =
    hasActiveTaskScope && (mode === "list" || mode === "kanban" || options.forceListData === true);
  const tasksQuery = useTasks(listFilters, { enabled: isListTab });
  const inboxQuery = useTaskInbox(inboxFilters, {
    enabled: hasActiveTaskScope && mode === "inbox",
  });
  const inboxBadgeQuery = useTaskInboxBadge(inboxBadgeFilters, { enabled: hasActiveTaskScope });
  const dashboard = useTasksDashboardPage(
    dashboardFilters,
    hasActiveTaskScope && mode === "dashboard"
  );

  const allTasks = tasksQuery.data ?? [];
  const visibleTasks = allTasks;
  const statusCounts = taskStatusCountsFromFacets(tasksQuery.facets);
  // Kanban shows top-level cards; nested subtasks stay behind their parent.
  const kanbanColumns: KanbanColumnGroup[] = groupTasksForKanban(
    buildTaskListTree(visibleTasks).roots
  );
  const ownerOptions = tasksQuery.facets.owners.map(facet => ({
    kind: facet.owner.kind,
    ref: facet.owner.ref,
  }));

  const effectiveSelectedTaskId = visibleTasks[0]?.id ?? null;
  const actions = useTasksPageActions();
  const loadMoreTasks = () => {
    void tasksQuery.fetchNextPage();
  };
  const loadMoreInbox = () => {
    void inboxQuery.fetchNextPage();
  };
  const retryTasks = () => {
    if (tasksQuery.isFetchNextPageError) {
      void tasksQuery.fetchNextPage();
      return;
    }
    void tasksQuery.refetch();
  };
  const retryInbox = () => {
    if (inboxQuery.isFetchNextPageError) {
      void inboxQuery.fetchNextPage();
      return;
    }
    void inboxQuery.refetch();
  };

  const hasListFilters = Boolean(
    statusFilter || ownerFilter || priorityFilter || routeSearchQuery.trim()
  );
  const isEmpty =
    hasActiveTaskScope &&
    !scopeLoading &&
    !scopeError &&
    !tasksQuery.isLoading &&
    !tasksQuery.error &&
    tasksQuery.total === 0 &&
    !hasListFilters;

  return {
    ...actions,
    ...dashboard,
    activeWorkspaceName: activeWorkspace?.name ?? null,
    dashboardError: scopeError ?? dashboard.dashboardError,
    dashboardLoading: scopeLoading || dashboard.dashboardLoading,
    effectiveSelectedTaskId,
    handleInboxLaneChange: (lane: InboxLaneFilter) =>
      updateSearch({ inboxLane: lane === "all" ? undefined : lane }),
    handleInboxPriorityChange: (priority: TaskPriority | null) =>
      updateSearch({ inboxPriority: priority ?? undefined }),
    handleInboxStatusChange: (status: TaskStatus | null) =>
      updateSearch({ inboxStatus: status ?? undefined }),
    handleInboxUnreadToggle: (unreadOnly: boolean) =>
      updateSearch({ inboxUnread: unreadOnly ? true : undefined }),
    handleOwnerChange: (owner: TaskFilterOwnerOption | null) =>
      updateSearch({ owner: owner ? taskOwnerFilterValue(owner) : undefined }),
    handlePriorityChange: (priority: TaskPriority | null) =>
      updateSearch({ priority: priority ?? undefined }),
    handleSortChange: (sort: TaskListSortKey) =>
      updateSearch({ sort: sort === "recent" ? undefined : sort }),
    handleStatusChange: (status: TaskStatus | null) =>
      updateSearch({ status: status ?? undefined }),
    hasMoreInbox: inboxQuery.hasNextPage,
    hasMoreTasks: tasksQuery.hasNextPage,
    inbox: inboxQuery.data ?? null,
    inboxError: scopeError ?? inboxQuery.error ?? null,
    inboxLaneFilter,
    inboxLoading: scopeLoading || (inboxQuery.isLoading && !inboxQuery.data),
    inboxPriorityFilter,
    inboxSearchQuery,
    inboxStatusFilter,
    inboxUnreadOnly,
    inboxUnreadCount: inboxBadgeQuery.data?.unread_total,
    inboxUpdatedAt: inboxQuery.dataUpdatedAt,
    isEmpty,
    isLoadingMoreInbox: inboxQuery.isFetchingNextPage,
    isLoadingMoreTasks: tasksQuery.isFetchingNextPage,
    kanbanColumns,
    listError: scopeError ?? tasksQuery.error ?? null,
    listLoading: scopeLoading || (tasksQuery.isLoading && allTasks.length === 0),
    listUpdatedAt: tasksQuery.dataUpdatedAt,
    loadMoreInbox,
    loadMoreTasks,
    mode,
    ownerFilter,
    ownerOptions,
    priorityFilter,
    retryInbox,
    retryTasks,
    searchQuery,
    setInboxSearchQuery,
    setSearchQuery,
    sortBy,
    statusCounts,
    statusFilter,
    hasActiveTaskScope,
    scopeError,
    scopeLoading,
    tasksCount: tasksQuery.dataUpdatedAt > 0 ? tasksQuery.total : undefined,
    visibleTasks,
  };
}

export { useTasksPage };
export type { InboxLaneFilter, UseTasksPageOptions };
