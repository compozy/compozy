import type { TaskDashboardFilter, TaskViewMode } from "../types";
import type { InboxLaneFilterId } from "@/systems/tasks/lib/inbox-grouping";

import type { TasksRouteSearch } from "../lib/task-location-search";
import { useTasksDashboardPage } from "./use-tasks-dashboard-page";
import { useTasksPageActions } from "./use-tasks-page-actions";
import { useTasksPageInbox, useTasksPageList } from "./use-tasks-page-reads";
import { useTasksPageScope } from "./use-tasks-page-scope";
import { useTasksPageSearch, type SearchChangeHandler } from "./use-tasks-page-search";

type InboxLaneFilter = InboxLaneFilterId;

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
  const { activeTaskScope, activeWorkspaceName, profile, ...scopeStatus } =
    useTasksPageScope(liveDataEnabled);
  const { includeLoop, routeInboxSearchQuery, routeSearchQuery, ...search } = useTasksPageSearch(
    routeSearch,
    options.onSearchChange
  );
  const { mode } = search;
  const scopeLive = liveDataEnabled && scopeStatus.hasActiveTaskScope;

  const list = useTasksPageList({
    activeTaskScope,
    enabled: scopeLive && listDataWanted(mode, options.forceListData),
    hasListFilters: search.hasListFilters,
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
  const dashboardFilters: TaskDashboardFilter = {
    scope: activeTaskScope?.scope,
    workspace: activeTaskScope?.workspace,
    worktree: activeTaskScope?.worktree,
  };
  const dashboard = useTasksDashboardPage(dashboardFilters, scopeLive && mode === "dashboard");
  const actions = useTasksPageActions();

  return {
    ...actions,
    ...dashboard,
    ...search,
    ...list,
    ...inbox,
    ...scopeStatus,
    activeWorkspaceName,
    dashboardError: scopeStatus.scopeError ?? dashboard.dashboardError,
    dashboardLoading: scopeStatus.scopeLoading || dashboard.dashboardLoading,
    profile,
  };
}

export { useTasksPage };
export type { InboxLaneFilter, UseTasksPageOptions };
