import { CancelledError, type QueryClient } from "@tanstack/react-query";

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

const ROUTE_QUERY_CANCEL_RETRIES = 3;

/**
 * `ensureQueryData` for a read the route genuinely awaits. A loader joins the
 * fetch a mounted observer may own; when that observer unmounts mid-flight
 * (a deferred window switch, a hidden deck body) TanStack cancels the shared
 * fetch and every joined promise rejects with `CancelledError`, which would
 * fail the whole route match. The read is still wanted, so ensure it again:
 * that joins a live successor or starts a fresh fetch. Bounded so repeated
 * cancellation still surfaces instead of looping.
 */
export async function ensureRouteQueryData<TData>(ensure: () => Promise<TData>): Promise<TData> {
  for (let attempt = 1; ; attempt += 1) {
    try {
      return await ensure();
    } catch (error) {
      if (!(error instanceof CancelledError) || attempt >= ROUTE_QUERY_CANCEL_RETRIES) {
        throw error;
      }
    }
  }
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
    ensureRouteQueryData(() => queryClient.ensureQueryData(workspacesListOptions())),
  ] as const);
}
