import type { QueryClient } from "@tanstack/react-query";

import { localProfileView } from "../stores/profile-view-store";
import { PERMANENT_PROFILE } from "./profile-rows";
import { activeWorkspaceStore } from "@/systems/workspace";

import { profileScopeParams, type ProfileScopeParams } from "./profile-scope";
import { profileKeys } from "./query-keys";
import type { ProfileLens, ProfileSelectionPayload, ProfileView } from "../types";

/**
 * The active view outside React — the same ladder `useActiveProfileView` walks.
 *
 * Route loaders prefetch before any component mounts, so they cannot read the
 * hook. Resolving from the same two sources keeps a preload's cache key equal to
 * the one its component will ask for; guessing would issue a request scoped to a
 * profile the operator is not in and file the answer under a key nobody reads.
 */
export function readProfileView(queryClient: QueryClient, lens: ProfileLens): ProfileView {
  const local = localProfileView(lens);
  if (local) return local;
  const remembered = queryClient.getQueryData<ProfileSelectionPayload>(profileKeys.selection(lens));
  return { kind: "profile", profile: remembered?.profile ?? PERMANENT_PROFILE };
}

export function readProfileScopeParams(
  queryClient: QueryClient,
  lens: ProfileLens
): ProfileScopeParams {
  return profileScopeParams(readProfileView(queryClient, lens));
}

/**
 * The lens outside React — the same slot `useProfileLens` picks.
 *
 * Route loaders run before any component mounts, so they read the shell's
 * persisted store directly rather than a hook. Falling back to the Global slot
 * matches the hook: a workspace that has not resolved yet is not a workspace.
 */
export function readProfileLens(): ProfileLens {
  const context = activeWorkspaceStore.getSnapshot().context;
  const workspaceId = context.selectedWorkspaceId;
  if (context.scope === "workspace" && workspaceId !== null && workspaceId !== "") {
    return { scope: "workspace", workspaceId };
  }
  return { scope: "global" };
}
