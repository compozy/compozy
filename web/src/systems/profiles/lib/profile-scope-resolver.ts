import type { QueryClient } from "@tanstack/react-query";

import { localProfileView } from "../stores/profile-view-store";
import { actingProfile, PERMANENT_PROFILE } from "./profile-rows";

import { profileScopeParams, type ProfileScopeParams } from "./profile-scope";
import { profileKeys } from "./query-keys";
import type { ProfileSelectionPayload, ProfileView } from "../types";

/**
 * The active view outside React — the same ladder `useActiveProfileView` walks.
 *
 * Route loaders prefetch before any component mounts, so they cannot read the
 * hook. Resolving from the same two sources keeps a preload's cache key equal to
 * the one its component will ask for; guessing would issue a request scoped to a
 * profile the operator is not in and file the answer under a key nobody reads.
 */
export function readProfileView(queryClient: QueryClient): ProfileView {
  const local = localProfileView();
  if (local) return local;
  return { kind: "profile", profile: readRememberedProfile(queryClient) ?? PERMANENT_PROFILE };
}

export function readProfileScopeParams(queryClient: QueryClient): ProfileScopeParams {
  return profileScopeParams(readProfileView(queryClient));
}

/** The profile a creation lands in right now, the same answer `useProfileReadScope` gives. */
export function readActingProfile(queryClient: QueryClient): string {
  return actingProfile(readProfileView(queryClient), readRememberedProfile(queryClient));
}

function readRememberedProfile(queryClient: QueryClient): string | undefined {
  const switches = queryClient
    .getMutationCache()
    .findAll({ mutationKey: profileKeys.selectionWrite() })
    .map((mutation): SelectionSwitch => ({
      status: mutation.state.status,
      view: mutation.state.variables as ProfileView | undefined,
    }));
  return rememberedAfterSwitches(
    switches,
    queryClient.getQueryData<ProfileSelectionPayload>(profileKeys.selection())?.profile
  );
}

/** One switch's write, as the mutation cache records it. */
export interface SelectionSwitch {
  status: "idle" | "pending" | "success" | "error";
  view: ProfileView | undefined;
}

/**
 * The remembered choice given this client's switch writes, oldest first. The newest concrete
 * switch that has not failed decides: while it is being written it is the remembered choice (a
 * reread landing before the write would answer with the previous one), and once written the
 * daemon's answer is. An older write still pending never overrides a newer one (#1194).
 */
export function rememberedAfterSwitches(
  switches: readonly SelectionSwitch[],
  cached: string | undefined
): string | undefined {
  for (let index = switches.length - 1; index >= 0; index -= 1) {
    const { status, view } = switches[index];
    if (view?.kind !== "profile" || status === "error" || status === "idle") continue;
    return status === "pending" ? view.profile : cached;
  }
  return cached;
}
