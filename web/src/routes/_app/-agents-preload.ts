import type { QueryClient } from "@tanstack/react-query";

import { sessionsListOptions } from "@/systems/session";

import { resolveActiveWorkspaceSelection, settleRouteQueries } from "./-route-preload";
import {
  agentAggregateOptions,
  agentCatalogOptions,
  type AgentCatalogStableFilter,
  agentDetailOptions,
} from "@/systems/agent";
import { settingsProvidersListOptions } from "@/systems/settings";
import {
  dataScopeFromSelector,
  focusedWorkspaceSelector,
  workspaceDetailOptions,
  workspaceSelectorOrAll,
} from "@/systems/workspace";
import { readActingProfile, readProfileScopeParams } from "@/systems/profiles";

async function resolveRouteScope(queryClient: QueryClient, routeWorkspaceId?: string) {
  const bound = routeWorkspaceId ? dataScopeFromSelector(routeWorkspaceId) : null;
  return bound ?? (await resolveActiveWorkspaceSelection(queryClient)).dataScope;
}

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
  name: string,
  routeWorkspaceId?: string
): Promise<void> {
  const dataScope = await resolveRouteScope(queryClient, routeWorkspaceId);
  if (!dataScope) return;
  const workspaceId = workspaceSelectorOrAll(dataScope);
  const profile = readActingProfile(queryClient);
  await settleRouteQueries([
    queryClient.query({ ...agentDetailOptions(name, workspaceId, profile), staleTime: "static" }),
    ...(dataScope.kind === "project"
      ? [
          queryClient.query({
            ...workspaceDetailOptions(dataScope.workspaceId),
            staleTime: "static",
          }),
        ]
      : []),
    queryClient.query({ ...settingsProvidersListOptions(), staleTime: "static" }),
  ]);
}

export async function preloadAgentDetailRoute(
  queryClient: QueryClient,
  name: string,
  routeWorkspaceId?: string
): Promise<void> {
  const dataScope = await resolveRouteScope(queryClient, routeWorkspaceId);
  if (!dataScope) return;
  const workspaceId = workspaceSelectorOrAll(dataScope);
  const profileScope = readProfileScopeParams(queryClient);
  const workspaceScope = workspaceId ? { workspace_id: workspaceId } : { all_workspaces: true };

  const profile = readActingProfile(queryClient);
  // These are the detail page's real consumers: exact catalog metrics and both
  // session shelves. Count probes and user-only lists are different cache entries.
  await settleRouteQueries([
    queryClient.query({ ...agentDetailOptions(name, workspaceId, profile), staleTime: "static" }),
    queryClient.infiniteQuery({
      ...agentCatalogOptions(workspaceId ?? "", { name, limit: 1, profile }),
      staleTime: "static",
    }),
    queryClient.infiniteQuery({
      ...sessionsListOptions({
        ...workspaceScope,
        agent: name,
        sort: "last_activity",
        ...profileScope,
      }),
      staleTime: "static",
    }),
    queryClient.infiniteQuery({
      ...sessionsListOptions({
        ...workspaceScope,
        agent: name,
        sort: "last_activity",
        archive: "only",
        ...profileScope,
      }),
      staleTime: "static",
    }),
  ]);
}
