import type { QueryClient } from "@tanstack/react-query";

import {
  selectActiveWorkspace,
  useActiveWorkspaceStore,
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
      ? useActiveWorkspaceStore.getState().selectedWorkspaceId
      : null;
  return selectActiveWorkspace(workspacesResult.value, selectedWorkspaceId)?.id ?? null;
}

export async function selectRouteWorkspaceForNavigation(
  queryClient: QueryClient,
  routeWorkspaceId: string
): Promise<void> {
  const [hydrationResult, workspacesResult] = await loadWorkspaceSelection(queryClient);
  if (hydrationResult.status === "rejected" || workspacesResult.status === "rejected") {
    return;
  }
  if (!workspacesResult.value.some(workspace => workspace.id === routeWorkspaceId)) {
    return;
  }
  useActiveWorkspaceStore.getState().setSelectedWorkspaceId(routeWorkspaceId);
}

function loadWorkspaceSelection(queryClient: QueryClient) {
  const hydration = useActiveWorkspaceStore.persist.hasHydrated()
    ? Promise.resolve()
    : useActiveWorkspaceStore.persist.rehydrate();

  return Promise.allSettled([
    hydration,
    queryClient.ensureQueryData(workspacesListOptions()),
  ] as const);
}
