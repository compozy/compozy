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
  return queryClient.getQueryData<ProfileSelectionPayload>(profileKeys.selection())?.profile;
}
