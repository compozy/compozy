import { useState } from "react";

import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { useTaskInbox, useTaskInboxBadge } from "./use-task-inbox";
import { useTasks } from "./use-tasks";
import type {
  TaskDashboardFilter,
  TaskInboxFilter,
  TaskListFilter,
  TaskListSortKey,
  TaskPriority,
  TaskRecordsFilter,
  TaskStatus,
  TaskViewMode,
} from "../types";
import { groupTasksForKanban } from "@/systems/tasks/lib/task-grouping";
import type { KanbanColumnGroup } from "@/systems/tasks/lib/task-grouping";
import type { InboxLaneFilterId } from "@/systems/tasks/lib/inbox-grouping";
import { taskStatusCountsFromFacets } from "@/systems/tasks/lib/task-list-query";
import {
  taskOwnerFilterFromValue,
  taskOwnerFilterValue,
  type TaskFilterOwnerOption,
} from "@/systems/tasks/lib/tasks-list-filters";

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
import { useActiveWorkspace, useActiveWorktree, useWorktrees } from "@/systems/workspace";
import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { useProfileReadScope } from "@/systems/profiles";

type InboxLaneFilter = InboxLaneFilterId;
const SEARCH_DEBOUNCE_MS = 200;

interface UseTasksPageOptions {
  /** Validated URL state. The route is the sole owner of catalog filters. */
  search?: TasksRouteSearch;
  onSearchChange?: (update: (current: TasksRouteSearch) => TasksRouteSearch) => void;
  forceListData?: boolean;
  liveDataEnabled?: boolean;
}

function resolveTaskScopeError(
  hasActiveTaskScope: boolean,
  scopeLoading: boolean,
  scopeSourceError: Error | null
): Error | null {
  if (hasActiveTaskScope || scopeLoading) return null;
  if (scopeSourceError) return scopeSourceError;
  return new Error("No active workspace is available for task scope.");
}

function useTasksPage(options: UseTasksPageOptions = {}) {
  const liveDataEnabled = options.liveDataEnabled ?? true;
  const workspace = useActiveWorkspace({ enabled: liveDataEnabled });
  const { activeWorkspace, activeWorkspaceId, scope } = workspace;
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
  // Ephemeral by contract (US-002.AC-3): local state, never route search, so it
  // survives neither a reload nor a shared link. The mode rides along so
  // switching surface resets it.
  const [reveal, setReveal] = useState<{ mode: TaskViewMode; records: TaskRecordsFilter }>({
    mode,
    records: "work",
  });
  if (reveal.mode !== mode) {
    setReveal({ mode, records: "work" });
  }
  const recordsFilter: TaskRecordsFilter = reveal.mode === mode ? reveal.records : "work";
  // The reveal is a list-surface affordance; kanban roots are work items only.
  const includeLoop = mode === "list" && recordsFilter === "loop";
  const updateSearch = (patch: Partial<TasksRouteSearch>) => {
    options.onSearchChange?.(current =>
      validateTasksSearch({
        ...current,
        ...patch,
      })
    );
  };
  const listSearch = useDebouncedInput({
    delayMs: SEARCH_DEBOUNCE_MS,
    externalValue: routeSearchQuery,
    onCommit: query => updateSearch({ query: query.trim() ? query : undefined }),
  });
  const inboxSearch = useDebouncedInput({
    delayMs: SEARCH_DEBOUNCE_MS,
    externalValue: routeInboxSearchQuery,
    onCommit: query => updateSearch({ inboxQuery: query.trim() ? query : undefined }),
  });

  // This window's worktree scope. Filtering happens server-side on the derived
  // active-run worktree id — a loaded page is never trimmed client-side.
  // Global cannot bind a worktree; the scope helper drops it on that branch.
  // The listing's profile axis: owner tags under the aggregate, and the profile
  // the empty state names.
  const profile = useProfileReadScope();
  const worktreeScopeId = useWorktreeScopeId();
  const worktreesQuery = useWorktrees(activeWorkspaceId, {
    enabled: liveDataEnabled && activeWorkspaceId !== null,
  });
  const worktreeSelection = useActiveWorktree(worktreeScopeId, worktreesQuery.data);
  const worktreeScopeResolved = scope !== "workspace" || worktreeSelection.resolved;
  const activeTaskScope = worktreeScopeResolved
    ? taskScopeForActiveWorkspace(
        scope,
        activeWorkspaceId,
        worktreeSelection.activeWorktree?.id ?? null
      )
    : null;
  const hasActiveTaskScope = activeTaskScope !== null;
  const scopeSourceError =
    scope === "workspace" && !activeWorkspaceId ? (workspace.error ?? null) : null;
  const scopeLoading =
    !hasActiveTaskScope &&
    !scopeSourceError &&
    (!worktreeScopeResolved || !workspace.hasHydrated || workspace.pending);
  const scopeError = resolveTaskScopeError(hasActiveTaskScope, scopeLoading, scopeSourceError);
  const listFilters: TaskListFilter = activeTaskScope
    ? taskListFilterFromRouteSearch(
        activeTaskScope,
        {
          ...routeSearch,
          query: routeSearchQuery || undefined,
        },
        { includeLoop }
      )
    : {};
  const dashboardFilters: TaskDashboardFilter = {
    scope: activeTaskScope?.scope,
    workspace: activeTaskScope?.workspace,
    worktree: activeTaskScope?.worktree,
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
    worktree: activeTaskScope?.worktree,
    limit: 1,
  };

  const isListTab =
    liveDataEnabled &&
    hasActiveTaskScope &&
    (mode === "list" || mode === "kanban" || options.forceListData === true);
  const tasksQuery = useTasks(listFilters, { enabled: isListTab });
  const inboxQuery = useTaskInbox(inboxFilters, {
    enabled: liveDataEnabled && hasActiveTaskScope && mode === "inbox",
  });
  const inboxBadgeQuery = useTaskInboxBadge(inboxBadgeFilters, {
    enabled: liveDataEnabled && hasActiveTaskScope,
  });
  const dashboard = useTasksDashboardPage(
    dashboardFilters,
    liveDataEnabled && hasActiveTaskScope && mode === "dashboard"
  );

  const allTasks = tasksQuery.data ?? [];
  const visibleTasks = allTasks;
  const statusCounts = taskStatusCountsFromFacets(tasksQuery.facets);
  // The server already cut the population, so the board renders exactly what the
  // list does — counts stay coherent by construction (US-001.AC-3).
  const kanbanColumns: KanbanColumnGroup[] =
    mode === "kanban" ? groupTasksForKanban(visibleTasks) : [];
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

  // The reveal counts as a filter here, or a revealed workspace with no loop
  // records would fall through to the zero-inventory template panel instead of
  // the filter-scoped message that explains what emptied the list.
  const hasListFilters = Boolean(
    statusFilter || ownerFilter || priorityFilter || routeSearchQuery.trim() || includeLoop
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
    profile,
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
    handleRecordsFilterChange: (records: TaskRecordsFilter) => setReveal({ mode, records }),
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
    inboxSearchQuery: inboxSearch.draftValue,
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
    recordsFilter,
    retryInbox,
    retryTasks,
    searchQuery: listSearch.draftValue,
    setInboxSearchQuery: inboxSearch.setDraftValue,
    setSearchQuery: listSearch.setDraftValue,
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
