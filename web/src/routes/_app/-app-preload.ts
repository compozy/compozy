import type { QueryClient } from "@tanstack/react-query";

import { agentCatalogOptions, agentDetailOptions, agentsListOptions } from "@/systems/agent";
import {
  homeActivityOptions,
  homeOverviewOptions,
  homeScopeForActiveWorkspace,
  homeWorkingNowSessionFilters,
  useHomePrefsStore,
} from "@/systems/dashboard";
import { networkStatusOptions } from "@/systems/network";
import { onboardingStatusOptions } from "@/systems/onboarding";
import { sessionsListOptions } from "@/systems/session";
import { statusOptions } from "@/systems/status";
import { taskDashboardOptions } from "@/systems/tasks";
import { workspaceDetailOptions, workspacesListOptions } from "@/systems/workspace";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";

export async function preloadAppRoute(queryClient: QueryClient): Promise<void> {
  const [[onboardingResult], workspaceId] = await Promise.all([
    Promise.allSettled([queryClient.ensureQueryData(onboardingStatusOptions())] as const),
    resolveActiveWorkspaceId(queryClient),
  ]);

  if (
    onboardingResult.status === "rejected" ||
    onboardingResult.value.completed !== true ||
    !workspaceId
  ) {
    return;
  }

  await settleRouteQueries([
    queryClient.ensureQueryData(agentsListOptions(workspaceId)),
    queryClient.ensureInfiniteQueryData(agentCatalogOptions(workspaceId, { limit: 1 })),
    queryClient.ensureQueryData(workspaceDetailOptions(workspaceId)),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({ workspace: workspaceId, state: "active", limit: 1 })
    ),
  ]);
}

export async function preloadHomeRoute(queryClient: QueryClient): Promise<void> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  if (!workspaceId) {
    return;
  }

  await preloadHomeWorkspace(queryClient, workspaceId);
}

/** Warm the dashboard projection for an explicitly owned workspace window. */
export async function preloadHomeWorkspace(
  queryClient: QueryClient,
  workspaceId: string
): Promise<void> {
  const usageWindow = useHomePrefsStore.getState().usageWindow;
  // Resolve scope from the awaited daemon status so preload and first paint use
  // the same workspace-specific query keys.
  const [statusResult, workspaces] = await Promise.all([
    queryClient.ensureQueryData(statusOptions()),
    queryClient.ensureQueryData(workspacesListOptions()),
  ]);
  const active = workspaces.find(workspace => workspace.id === workspaceId);
  const scope = homeScopeForActiveWorkspace(active, statusResult.daemon.user_home_dir);
  if (!scope) {
    // Never warm a global placeholder while workspace scope is undetermined.
    return;
  }
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

export async function preloadAgentDetailRoute(
  queryClient: QueryClient,
  name: string
): Promise<void> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  if (!workspaceId) {
    return;
  }

  await settleRouteQueries([
    queryClient.ensureQueryData(agentDetailOptions(name, workspaceId)),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({
        workspace: workspaceId,
        agent: name,
        type: "user",
        sort: "last_activity",
      })
    ),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({
        workspace: workspaceId,
        agent: name,
        state: "active",
        type: "user",
        limit: 1,
      })
    ),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({
        workspace: workspaceId,
        agent: name,
        type: "user",
        resumable: true,
        limit: 1,
      })
    ),
  ]);
}
