import type { QueryClient } from "@tanstack/react-query";

import {
  isProfileViewStoreHydrated,
  profileViewKey,
  readProfileScopeParams,
  readProfileView,
} from "@/systems/profiles";
import {
  activeWorkspaceStore,
  isActiveWorkspaceStoreHydrated,
  ownerDataScope,
  resolveActiveWorkspace,
  sameDataScope,
  workspacesListOptions,
} from "@/systems/workspace";

import type { SessionPayload } from "../types";
import { sessionScopedDetailOptions } from "./query-options";
import { sessionTranscriptOptions } from "./session-transcript-options";

const SPECULATIVE_TRANSCRIPT_GC_TIME_MS = 60_000;
type SessionEntryTarget = Pick<SessionPayload, "id" | "workspace_id" | "profile_name">;

/** Warm only the selected catalog row; hover never changes the desktop or profile. */
export async function preloadSessionEntry(queryClient: QueryClient, target: SessionEntryTarget) {
  if (!sessionEntryWithinScope(queryClient, target)) return;
  const profile = profileViewKey(readProfileView(queryClient));
  const session = await queryClient.query(
    sessionScopedDetailOptions(target.id, readProfileScopeParams(queryClient))
  );
  // A switch or a stale row cannot widen speculative transcript access while the read is in flight.
  if (
    profileViewKey(readProfileView(queryClient)) !== profile ||
    session.id !== target.id ||
    (session.workspace_id ?? "") !== (target.workspace_id ?? "") ||
    !sessionEntryWithinScope(queryClient, session)
  ) {
    return;
  }
  await queryClient.infiniteQuery({
    ...sessionTranscriptOptions(session.workspace_id ?? "", session.id),
    // A populated transcript belongs to its stream/mounted observer. Hover only fills a cold head,
    // never re-fetches every older page retained from a previous visit.
    staleTime: "static",
    // Never-opened transcript previews expire promptly; a mounted reader restores its normal policy.
    gcTime: SPECULATIVE_TRANSCRIPT_GC_TIME_MS,
  });
}

function sessionEntryWithinScope(queryClient: QueryClient, session: SessionEntryTarget): boolean {
  if (!session.id || !isActiveWorkspaceStoreHydrated() || !isProfileViewStoreHydrated())
    return false;
  const view = readProfileView(queryClient);
  if (view.kind === "profile" && session.profile_name !== view.profile) return false;
  const workspaces = queryClient.getQueryData(workspacesListOptions().queryKey);
  const { breadthMode, focus } = activeWorkspaceStore.getSnapshot().context;
  const { dataScope } = resolveActiveWorkspace({
    workspaces: workspaces ?? [],
    breadthMode,
    focus,
    pending: workspaces === undefined,
  });
  return (
    dataScope !== null &&
    (dataScope.kind === "all" || sameDataScope(dataScope, ownerDataScope(session.workspace_id)))
  );
}
