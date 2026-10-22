import type { QueryClient } from "@tanstack/react-query";

import {
  activeWorkspaceStore,
  isActiveWorkspaceStoreHydrated,
  rehydrateActiveWorkspaceStore,
  resolveActiveWorkspace,
  USER_FOCUS,
  workspacesListOptions,
  type ActiveWorkspaceResolution,
} from "@/systems/workspace";

/**
 * Route preloads are opportunistic because their pages own loading and error
 * presentation through TanStack Query. The queries start before the route
 * mounts, but their settlement must not delay the page's loading UI. Rejections
 * remain recorded in the query cache instead of reaching the route boundary.
 */
export function settleRouteQueries(queries: readonly Promise<unknown>[]): Promise<void> {
  void Promise.allSettled(queries);
  return Promise.resolve();
}

export async function resolveActiveWorkspaceSelection(
  queryClient: QueryClient
): Promise<ActiveWorkspaceResolution> {
  const [hydrationResult, workspacesResult] = await loadWorkspaceSelection(queryClient);

  if (workspacesResult.status === "rejected") {
    throw workspacesResult.reason;
  }

  const hydrated = hydrationResult.status === "fulfilled";
  const context = activeWorkspaceStore.getSnapshot().context;

  return resolveActiveWorkspace({
    workspaces: workspacesResult.value,
    breadthMode: hydrated ? context.breadthMode : "all",
    focus: hydrated ? context.focus : USER_FOCUS,
  });
}

export async function resolveActiveWorkspaceId(queryClient: QueryClient): Promise<string | null> {
  const resolution = await resolveActiveWorkspaceSelection(queryClient);
  return resolution.runtimeWorkspaceId;
}

/** Partition that owns the mounted desktop: the focused project, `user`, or `global`. */
export async function resolveDesktopWorkspaceId(queryClient: QueryClient): Promise<string | null> {
  const resolution = await resolveActiveWorkspaceSelection(queryClient);
  return resolution.desktopWorkspaceId;
}

function loadWorkspaceSelection(queryClient: QueryClient) {
  const hydration = isActiveWorkspaceStoreHydrated()
    ? Promise.resolve()
    : rehydrateActiveWorkspaceStore();

  return Promise.allSettled([
    hydration,
    queryClient.ensureQueryData(workspacesListOptions()),
  ] as const);
}
