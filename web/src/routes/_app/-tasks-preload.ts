import type { QueryClient } from "@tanstack/react-query";

import { resolveActiveWorkspaceSelection, settleRouteQueries } from "./-route-preload";
import { schedulerBacklogOptions, schedulerStatusOptions } from "@/systems/scheduler";
import {
  parseTasksSurfaceMode,
  taskDashboardOptions,
  taskInboxBadgeOptions,
  taskListFilterFromRouteSearch,
  taskDetailOptions,
  taskRunDetailOptions,
  taskRunsOptions,
  taskScopeForActiveWorkspace,
  tasksListOptions,
  type TasksRouteSearch,
} from "@/systems/tasks";
import { readProfileLens, readProfileScopeParams } from "@/systems/profiles";

export async function preloadTasksRoute(
  queryClient: QueryClient,
  search: TasksRouteSearch
): Promise<void> {
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());
  const mode = parseTasksSurfaceMode(search);
  const resolution = await resolveActiveWorkspaceSelection(queryClient);
  const scope = taskScopeForActiveWorkspace(resolution.scope, resolution.activeWorkspaceId);
  if (!scope) {
    return;
  }

  const queries: Promise<unknown>[] = [
    queryClient.ensureInfiniteQueryData(
      taskInboxBadgeOptions({
        scope: scope.scope,
        workspace: scope.workspace,
        limit: 1,
        ...profileScope,
      })
    ),
  ];
  if (mode === "dashboard") {
    queries.unshift(
      queryClient.ensureQueryData(
        taskDashboardOptions({ scope: scope.scope, workspace: scope.workspace, ...profileScope })
      ),
      queryClient.ensureQueryData(schedulerStatusOptions()),
      queryClient.ensureQueryData(
        schedulerBacklogOptions({
          include_paused: true,
          limit: 5,
          scope: scope.scope,
          workspace: scope.workspace,
        })
      )
    );
  } else if (mode === "inbox") {
    // The inbox query is stale-on-read and owns its mount refetch. Preloading it
    // would make hover intent perform an unconditional request and then refetch
    // again when the route mounts.
  } else {
    queries.unshift(
      queryClient.ensureInfiniteQueryData(
        tasksListOptions({ ...taskListFilterFromRouteSearch(scope, search), ...profileScope })
      )
    );
  }
  await settleRouteQueries(queries);
}

/**
 * Warms a task detail on hover intent: the record and its run history, the two
 * reads the detail page renders first. A missing or foreign task resolves to
 * nothing so the route's own error state stays authoritative.
 */
export async function preloadTaskDetailRoute(queryClient: QueryClient, id: string): Promise<void> {
  if (!id) return;
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());
  await settleRouteQueries([
    queryClient.ensureQueryData(taskDetailOptions(id, profileScope)).catch(() => null),
    queryClient.ensureQueryData(taskRunsOptions(id, profileScope)),
  ]);
}

/** Warms a task run page: the run itself plus its parent task and sibling runs. */
export async function preloadTaskRunRoute(
  queryClient: QueryClient,
  taskId: string,
  runId: string
): Promise<void> {
  if (!runId) return;
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());
  await settleRouteQueries([
    queryClient.ensureQueryData(taskRunDetailOptions(runId, profileScope)).catch(() => null),
    preloadTaskDetailRoute(queryClient, taskId),
  ]);
}
