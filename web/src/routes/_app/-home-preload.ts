import type { QueryClient } from "@tanstack/react-query";

import { agentsListOptions } from "@/systems/agent/lib/query-options";
import { homePrefsStore } from "@/systems/dashboard/hooks/use-home-prefs-store";
import { homeScopeForActiveWorkspace } from "@/systems/dashboard/lib/home-scope";
import { homeWorkingNowSessionFilters } from "@/systems/dashboard/lib/home-working-now-query";
import { homeActivityOptions, homeOverviewOptions } from "@/systems/dashboard/lib/query-options";
import { networkStatusOptions } from "@/systems/network/lib/query-options";
import { sessionsListOptions } from "@/systems/session";
import { statusOptions } from "@/systems/status/lib/query-options";
import { taskDashboardOptions } from "@/systems/tasks/lib/query-options";
import { workspacesListOptions } from "@/systems/workspace/lib/query-options";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";

export async function preloadHomeRoute(queryClient: QueryClient): Promise<void> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  if (!workspaceId) return;
  await preloadHomeWorkspace(queryClient, workspaceId);
}

export async function preloadHomeWorkspace(
  queryClient: QueryClient,
  workspaceId: string
): Promise<void> {
  const usageWindow = homePrefsStore.getSnapshot().context.usageWindow;
  const [statusResult, workspaces] = await Promise.all([
    queryClient.ensureQueryData(statusOptions()),
    queryClient.ensureQueryData(workspacesListOptions()),
  ]);
  const active = workspaces.find(workspace => workspace.id === workspaceId);
  const scope = homeScopeForActiveWorkspace(active, statusResult.daemon.user_home_dir);
  if (!scope) return;

  await settleRouteQueries([
    queryClient.ensureQueryData(agentsListOptions(workspaceId)),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({
        ...homeWorkingNowSessionFilters(),
        ...(scope.workspaceParam ? { workspace: scope.workspaceParam } : {}),
      })
    ),
    queryClient.ensureQueryData(
      homeOverviewOptions({ workspace: scope.workspaceParam || undefined, usageWindow })
    ),
    queryClient.ensureQueryData(
      homeActivityOptions({ workspace_id: scope.workspaceParam || undefined })
    ),
    queryClient.ensureQueryData(taskDashboardOptions(scope.taskScope)),
    queryClient.ensureQueryData(networkStatusOptions()),
  ]);
}
