import { CancelledError, type QueryClient } from "@tanstack/react-query";

import {
  ensureRouteQueryData,
  resolveActiveWorkspaceId,
  settleRouteQueries,
} from "./-route-preload";
import { agentCatalogOptions, agentsListOptions } from "@/systems/agent";
import { onboardingStatusOptions } from "@/systems/onboarding";
import { providerUsageOptions } from "@/systems/provider-usage";
import { sessionsListOptions } from "@/systems/session";
import {
  isActiveWorkspaceStoreHydrated,
  rehydrateActiveWorkspaceStore,
  workspaceDetailOptions,
} from "@/systems/workspace";
import {
  isProfileViewStoreHydrated,
  localProfileView,
  profileSelectionOptions,
  readActingProfile,
  readProfileScopeParams,
  rehydrateProfileViewStore,
} from "@/systems/profiles";

/** Resolve identity before parallel loaders or mounted consumers can read work. */
export async function prepareAppProfile(queryClient: QueryClient): Promise<void> {
  if (!isActiveWorkspaceStoreHydrated()) {
    await rehydrateActiveWorkspaceStore();
  }
  // A persisted All profiles view must win before any scoped read is issued.
  if (!isProfileViewStoreHydrated()) {
    await rehydrateProfileViewStore();
  }
  const options = profileSelectionOptions();
  for (;;) {
    // The aggregate still needs the remembered creation destination.
    if (localProfileView()?.kind === "profile") return;
    try {
      await queryClient.fetchQuery(options);
      return;
    } catch (error) {
      // A joined TanStack fetch can expose the old promise's silent cancellation
      // when reconciliation replaces it. Follow the replacement, never cached
      // identity or a cancelled read without a live successor.
      if (
        !(error instanceof CancelledError) ||
        !error.silent ||
        queryClient.getQueryState(options.queryKey)?.fetchStatus !== "fetching"
      ) {
        throw error;
      }
    }
  }
}

export async function preloadAppRoute(queryClient: QueryClient): Promise<void> {
  const [onboardingResult, workspaceResult] = await Promise.allSettled([
    queryClient.ensureQueryData(onboardingStatusOptions()),
    resolveActiveWorkspaceId(queryClient),
  ]);

  if (
    onboardingResult.status === "rejected" ||
    workspaceResult.status === "rejected" ||
    onboardingResult.value.completed !== true ||
    !workspaceResult.value
  ) {
    return;
  }
  const workspaceId = workspaceResult.value;
  const profileScope = readProfileScopeParams(queryClient);

  const profile = readActingProfile(queryClient);
  // The menubar mounts its usage tray only for a profile with usage targets. Reading the stored
  // envelope before the shell renders lets the tray land with the shell rather than appear (or
  // vanish) beside the controls next to it. Settled: a failed read never holds the desktop.
  if (profile !== "") {
    await Promise.allSettled([
      ensureRouteQueryData(() => queryClient.ensureQueryData(providerUsageOptions(profile))),
    ]);
  }
  await settleRouteQueries([
    queryClient.ensureQueryData(agentsListOptions(workspaceId, profile)),
    queryClient.ensureInfiniteQueryData(agentCatalogOptions(workspaceId, { limit: 1, profile })),
    queryClient.ensureQueryData(workspaceDetailOptions(workspaceId)),
    queryClient.ensureInfiniteQueryData(
      sessionsListOptions({ workspace_id: workspaceId, state: "active", limit: 1, ...profileScope })
    ),
  ]);
}
