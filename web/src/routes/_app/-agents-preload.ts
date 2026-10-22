import type { QueryClient } from "@tanstack/react-query";

import { sessionsListOptions } from "@/systems/session";

import {
  resolveActiveWorkspaceId,
  resolveActiveWorkspaceSelection,
  settleRouteQueries,
} from "./-route-preload";
import {
  agentAggregateOptions,
  agentCatalogOptions,
  type AgentCatalogStableFilter,
  agentDetailOptions,
} from "@/systems/agent";
import { settingsProvidersListOptions } from "@/systems/settings";
import { focusedWorkspaceSelector, workspaceDetailOptions } from "@/systems/workspace";
import { readActingProfile, readProfileScopeParams } from "@/systems/profiles";

export async function preloadAgentsRoute(
  queryClient: QueryClient,
  filters: AgentCatalogStableFilter
): Promise<void> {
  const dataScope = (await resolveActiveWorkspaceSelection(queryClient))?.dataScope ?? null;
  if (!dataScope) return;
  const profile = readActingProfile(queryClient);
  // All projects reads the aggregate (User + every project); otherwise the
  // project's catalog, or User's through `@user`.
  await settleRouteQueries([
    dataScope.kind === "all"
      ? queryClient.ensureInfiniteQueryData(agentAggregateOptions([], { ...filters, profile }))
      : queryClient.ensureInfiniteQueryData(
          agentCatalogOptions(focusedWorkspaceSelector(dataScope), { ...filters, profile })
        ),
  ]);
}

export async function preloadAgentSettingsRoute(
  queryClient: QueryClient,
  name: string
): Promise<void> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  const profile = readActingProfile(queryClient);
  await settleRouteQueries([
    queryClient.ensureQueryData(agentDetailOptions(name, workspaceId, profile)),
    ...(workspaceId ? [queryClient.ensureQueryData(workspaceDetailOptions(workspaceId))] : []),
    queryClient.ensureQueryData(settingsProvidersListOptions()),
  ]);
}

export async function preloadAgentDetailRoute(
  queryClient: QueryClient,
  name: string
): Promise<void> {
  const profileScope = readProfileScopeParams(queryClient);
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  const workspaceScope = workspaceId ? { workspace_id: workspaceId } : { all_workspaces: true };

  const profile = readActingProfile(queryClient);
  await settleRouteQueries([
    queryClient.ensureQueryData(agentDetailOptions(name, workspaceId, profile)),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({
        ...workspaceScope,
        agent: name,
        type: "user",
        sort: "last_activity",
        ...profileScope,
      })
    ),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({
        ...workspaceScope,
        agent: name,
        state: "active",
        type: "user",
        limit: 1,
        ...profileScope,
      })
    ),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({
        ...workspaceScope,
        agent: name,
        type: "user",
        resumable: true,
        limit: 1,
        ...profileScope,
      })
    ),
  ]);
}
