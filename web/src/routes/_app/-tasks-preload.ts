import type { QueryClient } from "@tanstack/react-query";
import type { TasksRouteSearch } from "@/systems/tasks";

import { statusOptions } from "@/systems/status";
import { schedulerBacklogOptions, schedulerStatusOptions } from "@/systems/scheduler";
import { taskDashboardOptions, taskInboxBadgeOptions, tasksListOptions } from "@/systems/tasks";
import { workspacesListOptions } from "@/systems/workspace";
import { defaultTaskCatalogFilter, taskScopeForActiveWorkspace } from "@/systems/tasks";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";

export async function preloadTasksRoute(
  queryClient: QueryClient,
  mode: TasksRouteSearch["mode"]
): Promise<void> {
  const [workspaceId, statusResult, workspacesResult] = await Promise.all([
    resolveActiveWorkspaceId(queryClient),
    queryClient.ensureQueryData(statusOptions()).catch(() => null),
    queryClient.ensureQueryData(workspacesListOptions()).catch(() => []),
  ]);
  const activeWorkspace = workspacesResult.find(workspace => workspace.id === workspaceId);
  const scope = taskScopeForActiveWorkspace(activeWorkspace, statusResult?.daemon.user_home_dir);
  if (!scope) {
    return;
  }

  const queries: Promise<unknown>[] = [
    queryClient.ensureInfiniteQueryData(
      taskInboxBadgeOptions({ scope: scope.scope, workspace: scope.workspace, limit: 1 })
    ),
  ];
  if (mode === "dashboard") {
    queries.unshift(
      queryClient.ensureQueryData(
        taskDashboardOptions({ scope: scope.scope, workspace: scope.workspace })
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
  } else if (mode !== "inbox") {
    queries.unshift(
      queryClient.ensureInfiniteQueryData(tasksListOptions(defaultTaskCatalogFilter(scope)))
    );
  }
  await settleRouteQueries(queries);
}
