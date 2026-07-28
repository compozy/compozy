import type { QueryClient } from "@tanstack/react-query";

import {
  agentCatalogOptions,
  agentDetailOptions,
  type AgentCatalogStableFilter,
} from "@/systems/agent";
import { settingsProvidersListOptions } from "@/systems/settings";
import { workspaceDetailOptions } from "@/systems/workspace";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";

export async function preloadAgentsRoute(
  queryClient: QueryClient,
  filters: AgentCatalogStableFilter
): Promise<void> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  if (!workspaceId) return;
  await settleRouteQueries([
    queryClient.ensureInfiniteQueryData(agentCatalogOptions(workspaceId, filters)),
  ]);
}

export async function preloadAgentSettingsRoute(
  queryClient: QueryClient,
  name: string
): Promise<void> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  if (!workspaceId) return;
  await settleRouteQueries([
    queryClient.ensureQueryData(agentDetailOptions(name, workspaceId)),
    queryClient.ensureQueryData(workspaceDetailOptions(workspaceId)),
    queryClient.ensureQueryData(settingsProvidersListOptions()),
  ]);
}
