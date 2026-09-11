import type { QueryClient } from "@tanstack/react-query";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";
import { agentCatalogOptions, agentsListOptions } from "@/systems/agent";
import { onboardingStatusOptions } from "@/systems/onboarding";
import { sessionsListOptions } from "@/systems/session";
import { workspaceDetailOptions } from "@/systems/workspace";
import {
  actingProfile,
  readProfileLens,
  readProfileView,
  readProfileScopeParams,
} from "@/systems/profiles";

export async function preloadAppRoute(queryClient: QueryClient): Promise<void> {
  const [onboardingResult, workspaceResult] = await Promise.allSettled([
    queryClient.ensureQueryData(onboardingStatusOptions()),
    resolveActiveWorkspaceId(queryClient),
  ]);

  if (
    onboardingResult.status === "rejected" ||
    workspaceResult.status === "rejected" ||
    onboardingResult.value.completed !== true ||
    !workspaceResult.value
  ) {
    return;
  }
  const workspaceId = workspaceResult.value;
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());

  const profile = actingProfile(readProfileView(queryClient, readProfileLens()));
  await settleRouteQueries([
    queryClient.ensureQueryData(agentsListOptions(workspaceId, profile)),
    queryClient.ensureInfiniteQueryData(agentCatalogOptions(workspaceId, { limit: 1, profile })),
    queryClient.ensureQueryData(workspaceDetailOptions(workspaceId)),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({ workspace_id: workspaceId, state: "active", limit: 1, ...profileScope })
    ),
  ]);
}
