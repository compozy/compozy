import type { QueryClient } from "@tanstack/react-query";

import { agentsListOptions } from "@/systems/agent/lib/query-options";
import { homePrefsStore } from "@/systems/dashboard/hooks/use-home-prefs-store";
import { homeScopeForActiveWorkspace } from "@/systems/dashboard/lib/home-scope";
import { homeWorkingNowSessionFilters } from "@/systems/dashboard/lib/home-working-now-query";
import { homeActivityOptions, homeOverviewOptions } from "@/systems/dashboard/lib/query-options";
import { networkStatusOptions } from "@/systems/network/lib/query-options";
import { sessionsListOptions } from "@/systems/session";
import { taskDashboardOptions } from "@/systems/tasks/lib/query-options";

import { resolveActiveWorkspaceSelection, settleRouteQueries } from "./-route-preload";
import { readProfileLens, readProfileScopeParams } from "@/systems/profiles";

export async function preloadHomeRoute(queryClient: QueryClient): Promise<void> {
  await preloadHomeWorkspace(queryClient);
}

export async function preloadHomeWorkspace(queryClient: QueryClient): Promise<void> {
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());
  const usageWindow = homePrefsStore.getSnapshot().context.usageWindow;
  let resolution: Awaited<ReturnType<typeof resolveActiveWorkspaceSelection>>;
  try {
    resolution = await resolveActiveWorkspaceSelection(queryClient);
  } catch {
    // The query cache owns the failure state rendered by the page. Route
    // preloading is opportunistic and must not turn that failure into a
    // navigation error.
    return;
  }
  const scope = homeScopeForActiveWorkspace(resolution.scope, resolution.activeWorkspaceId);
  if (!scope) return;
  const agentWorkspaceId = resolution.runtimeWorkspaceId;
  if (!agentWorkspaceId) return;

  await settleRouteQueries([
    queryClient.ensureQueryData(agentsListOptions(agentWorkspaceId)),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({
        ...homeWorkingNowSessionFilters(),
        ...(scope.workspaceParam
          ? { workspace_id: scope.workspaceParam }
          : { all_workspaces: true }),
        ...profileScope,
      })
    ),
    queryClient.ensureQueryData(
      homeOverviewOptions({
        workspace: scope.workspaceParam || undefined,
        usageWindow,
        ...("all_profiles" in profileScope
          ? { allProfiles: true }
          : { profile: profileScope.profile }),
      })
    ),
    queryClient.ensureQueryData(
      homeActivityOptions({ workspace_id: scope.workspaceParam || undefined, ...profileScope })
    ),
    queryClient.ensureQueryData(taskDashboardOptions({ ...scope.taskScope, ...profileScope })),
    queryClient.ensureQueryData(networkStatusOptions()),
  ]);
}
