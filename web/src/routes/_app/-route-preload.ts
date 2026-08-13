import type { QueryClient } from "@tanstack/react-query";

import { statusOptions } from "@/systems/status";
import {
  activeWorkspaceStore,
  isActiveWorkspaceStoreHydrated,
  rehydrateActiveWorkspaceStore,
  resolveActiveWorkspace,
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
  const [hydrationResult, workspacesResult, statusResult] =
    await loadWorkspaceSelection(queryClient);

  if (workspacesResult.status === "rejected") {
    throw workspacesResult.reason;
  }
  // Without `$HOME` the home row cannot be told apart from a project workspace,
  // so a failed status read is a failed resolution — never a silent guess.
  if (statusResult.status === "rejected") {
    throw statusResult.reason;
  }

  const hydrated = hydrationResult.status === "fulfilled";
  const context = activeWorkspaceStore.getSnapshot().context;
  const userHomeDir = statusResult.value.daemon.user_home_dir;

  return resolveActiveWorkspace({
    workspaces: workspacesResult.value,
    userHomeDir,
    scope: hydrated ? context.scope : "global",
    selectedWorkspaceId: hydrated ? context.selectedWorkspaceId : null,
  });
}

export async function resolveActiveWorkspaceId(queryClient: QueryClient): Promise<string | null> {
  const resolution = await resolveActiveWorkspaceSelection(queryClient);
  return resolution.runtimeWorkspaceId;
}

function loadWorkspaceSelection(queryClient: QueryClient) {
  const hydration = isActiveWorkspaceStoreHydrated()
    ? Promise.resolve()
    : rehydrateActiveWorkspaceStore();

  return Promise.allSettled([
    hydration,
    queryClient.ensureQueryData(workspacesListOptions()),
    queryClient.ensureQueryData(statusOptions()),
  ] as const);
}
