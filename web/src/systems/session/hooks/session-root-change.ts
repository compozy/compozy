import type { QueryClient, QueryKey } from "@tanstack/react-query";

import { filesKeys, requestFreshListings } from "@/systems/files";

import { sessionKeys } from "../lib/query-keys";
import { sessionChangesKeys } from "../lib/session-changes-query";
import { resetSessionReviewRoot } from "./use-session-review-panel-state";

/**
 * The session now runs in another checkout (an in-place worktree change, or a catalog upsert
 * with `execution_location_changed`). Its file, search and Changes reads are keyed by session,
 * not by checkout, so they are dropped and refetched rather than shown from the old root, and
 * the Review panel's root-bound state starts over.
 */
export async function resetSessionRootReads(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string,
  isOpen: () => boolean = () => true
): Promise<void> {
  resetSessionReviewRoot(sessionId);
  // The new checkout's listings must not come from the old root's status snapshot.
  requestFreshListings(queryClient, { kind: "session", workspaceId, sessionId });
  const dropped: QueryKey[] = [
    filesKeys.root({ kind: "session", workspaceId, sessionId }),
    sessionChangesKeys.session(workspaceId, sessionId),
  ];
  const mentions = sessionKeys.fileSearchRoot(workspaceId, sessionId);
  await Promise.all(
    [...dropped, mentions].map(queryKey => queryClient.cancelQueries({ queryKey }))
  );
  if (!isOpen()) return;
  await Promise.all([
    ...dropped.map(queryKey => queryClient.resetQueries({ queryKey })),
    // The mention picker keeps its last answer until the new root's one arrives.
    queryClient.invalidateQueries({ queryKey: mentions }),
  ]);
}
