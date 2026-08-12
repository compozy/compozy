import type { QueryClient } from "@tanstack/react-query";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";
import { agentCatalogOptions, agentsListOptions } from "@/systems/agent";
import { onboardingStatusOptions } from "@/systems/onboarding";
import { sessionsListOptions } from "@/systems/session";
import { workspaceDetailOptions } from "@/systems/workspace";

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
