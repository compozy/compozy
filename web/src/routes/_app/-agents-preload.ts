import type { QueryClient } from "@tanstack/react-query";

import { sessionsListOptions } from "@/systems/session";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";
import {
  agentCatalogOptions,
  type AgentCatalogStableFilter,
  agentDetailOptions,
} from "@/systems/agent";
import { settingsProvidersListOptions } from "@/systems/settings";
import { workspaceDetailOptions } from "@/systems/workspace";

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

export async function preloadAgentDetailRoute(
  queryClient: QueryClient,
  name: string
): Promise<void> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  if (!workspaceId) return;

  await settleRouteQueries([
    queryClient.ensureQueryData(agentDetailOptions(name, workspaceId)),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({
        workspace_id: workspaceId,
        agent: name,
        type: "user",
        sort: "last_activity",
      })
    ),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({
        workspace_id: workspaceId,
        agent: name,
        state: "active",
        type: "user",
        limit: 1,
      })
    ),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({
        workspace_id: workspaceId,
        agent: name,
        type: "user",
        resumable: true,
        limit: 1,
      })
    ),
  ]);
}
