import type { QueryClient } from "@tanstack/react-query";

import {
  loopAnnotationsOptions,
  loopConfigOptions,
  loopDetailOptions,
  loopRunDetailOptions,
  loopRunsOptions,
  loopsCatalogOptions,
  type LoopCatalogStableFilter,
  type LoopRunsFilter,
} from "@/systems/loops";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";

async function withActiveWorkspace(
  queryClient: QueryClient,
  preload: (workspaceId: string) => readonly Promise<unknown>[]
): Promise<void> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  if (!workspaceId) {
    return;
  }
  await settleRouteQueries(preload(workspaceId));
}

export function preloadLoopsRoute(
  queryClient: QueryClient,
  filters: LoopCatalogStableFilter
): Promise<void> {
  return withActiveWorkspace(queryClient, workspaceId => [
    queryClient.ensureInfiniteQueryData(loopsCatalogOptions(workspaceId, filters)),
  ]);
}

export function preloadLoopDetailRoute(queryClient: QueryClient, name: string): Promise<void> {
  return withActiveWorkspace(queryClient, workspaceId => [
    queryClient.ensureQueryData(loopDetailOptions(workspaceId, name)),
    queryClient.ensureQueryData(loopConfigOptions(workspaceId, name)),
    queryClient.ensureInfiniteQueryData(
      loopsCatalogOptions(workspaceId, { limit: 50, q: name, sort: "name" })
    ),
    queryClient.ensureQueryData(loopRunsOptions(workspaceId, { loop: name, limit: 5 })),
  ]);
}

export function preloadLoopRunFormRoute(queryClient: QueryClient, name: string): Promise<void> {
  return withActiveWorkspace(queryClient, workspaceId => [
    queryClient.ensureQueryData(loopDetailOptions(workspaceId, name)),
    queryClient.ensureQueryData(loopConfigOptions(workspaceId, name)),
  ]);
}

export function preloadLoopEditorRoute(queryClient: QueryClient, name: string): Promise<void> {
  return withActiveWorkspace(queryClient, workspaceId => [
    queryClient.ensureQueryData(loopDetailOptions(workspaceId, name)),
    queryClient.ensureQueryData(loopAnnotationsOptions(workspaceId, name)),
  ]);
}

export function preloadLoopConfigureRoute(queryClient: QueryClient, name: string): Promise<void> {
  return withActiveWorkspace(queryClient, workspaceId => [
    queryClient.ensureQueryData(loopDetailOptions(workspaceId, name)),
    queryClient.ensureQueryData(loopConfigOptions(workspaceId, name)),
  ]);
}

export function preloadLoopRunsRoute(
  queryClient: QueryClient,
  filters: LoopRunsFilter
): Promise<void> {
  return withActiveWorkspace(queryClient, workspaceId => [
    queryClient.ensureQueryData(loopRunsOptions(workspaceId, filters)),
  ]);
}

export async function preloadLoopRunDetailRoute(
  queryClient: QueryClient,
  runId: string
): Promise<void> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  if (!workspaceId) {
    return;
  }

  const [result] = await Promise.allSettled([
    queryClient.ensureQueryData(loopRunDetailOptions(workspaceId, runId)),
  ] as const);
  if (result.status === "rejected") {
    return;
  }

  const { executed_definition: executedDefinition, run } = result.value;
  if (!executedDefinition && run.loop_name) {
    await settleRouteQueries([
      queryClient.ensureQueryData(loopDetailOptions(workspaceId, run.loop_name)),
    ]);
  }
}
