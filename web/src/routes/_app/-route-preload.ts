import type { QueryClient } from "@tanstack/react-query";

import {
  activeWorkspaceStore,
  isActiveWorkspaceStoreHydrated,
  rehydrateActiveWorkspaceStore,
  selectActiveWorkspace,
  workspacesListOptions,
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

export async function resolveActiveWorkspaceId(queryClient: QueryClient): Promise<string | null> {
  const [hydrationResult, workspacesResult] = await loadWorkspaceSelection(queryClient);

  if (workspacesResult.status === "rejected") {
    return null;
  }

  const selectedWorkspaceId =
    hydrationResult.status === "fulfilled"
      ? activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId
      : null;
  return selectActiveWorkspace(workspacesResult.value, selectedWorkspaceId)?.id ?? null;
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
