import type { TaskViewMode } from "../types";
import type { InboxLaneFilterId } from "@/systems/tasks/lib/inbox-grouping";

import type { TasksRouteSearch } from "../lib/task-location-search";
import type { ActiveTaskScopeFilter } from "../lib/workspace-scope";
import { useTasksDashboardPage } from "./use-tasks-dashboard-page";
import { useTasksPageActions } from "./use-tasks-page-actions";
import { useTasksPageInbox, useTasksPageList } from "./use-tasks-page-reads";
import { useTasksPageScope } from "./use-tasks-page-scope";
import { useTasksPageSearch, type SearchChangeHandler } from "./use-tasks-page-search";

type InboxLaneFilter = InboxLaneFilterId;

/** Stable argument for the disabled dashboard read while the scope is unsettled. */
const UNSETTLED_TASK_SCOPE: ActiveTaskScopeFilter = {};

interface UseTasksPageOptions {
  /** Validated URL state. The route is the sole owner of catalog filters. */
  search?: TasksRouteSearch;
  onSearchChange?: SearchChangeHandler;
  forceListData?: boolean;
  liveDataEnabled?: boolean;
}

/** List data feeds the list and kanban surfaces, or any caller that forces it. */
function listDataWanted(mode: TaskViewMode, forceListData: boolean | undefined): boolean {
  return mode === "list" || mode === "kanban" || forceListData === true;
}

function useTasksPage(options: UseTasksPageOptions = {}) {
  const liveDataEnabled = options.liveDataEnabled ?? true;
  const routeSearch = options.search ?? {};
  const {
    activeTaskScope,
    activeWorkspaceName,
    listTaskScope,
    profile,
    projectBadgeOf,
    projectFilter,
    ...scopeStatus
  } = useTasksPageScope(liveDataEnabled);
  const { clearRouteListFilters, includeLoop, routeInboxSearchQuery, routeSearchQuery, ...search } =
    useTasksPageSearch(routeSearch, options.onSearchChange);
  const { mode } = search;
  const scopeLive = liveDataEnabled && scopeStatus.hasActiveTaskScope;

  const hasListFilters = search.hasListFilters || projectFilter.values.length > 0;
  const list = useTasksPageList({
    activeTaskScope: listTaskScope,
    enabled: scopeLive && listDataWanted(mode, options.forceListData),
    hasListFilters,
    includeLoop,
    mode,
    routeSearch,
    routeSearchQuery,
    scopeStatus,
  });
  const inbox = useTasksPageInbox({
    activeTaskScope,
    badgeEnabled: scopeLive,
    enabled: scopeLive && mode === "inbox",
    routeInboxSearchQuery,
    routeSearch,
    scopeStatus,
  });
  const dashboard = useTasksDashboardPage(
    activeTaskScope ?? UNSETTLED_TASK_SCOPE,
    scopeLive && mode === "dashboard"
  );
  const actions = useTasksPageActions();

  return {
    ...actions,
    ...dashboard,
    ...search,
    ...list,
    ...inbox,
    ...scopeStatus,
    hasListFilters,
    activeWorkspaceName,
    projectBadgeOf,
    projectFilter,
    clearListFilters: () => {
      clearRouteListFilters();
      projectFilter.clear();
    },
    dashboardError: scopeStatus.scopeError ?? dashboard.dashboardError,
    dashboardLoading: scopeStatus.scopeLoading || dashboard.dashboardLoading,
    profile,
  };
}

export { useTasksPage };
export type { InboxLaneFilter, UseTasksPageOptions };
