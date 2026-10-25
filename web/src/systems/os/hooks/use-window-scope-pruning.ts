import type { UseQueryResult } from "@tanstack/react-query";
import { useEffect } from "react";

import { pruneSessionSidebarScopes } from "@/systems/session";
import { pruneWorktreeScopes } from "@/systems/workspace";

import type { WindowManagerSnapshot } from "../lib/window-manager-types";

/**
 * Membership-only projection for the chrome read: geometry revisions must not
 * re-render the shell, and pruning only needs the live window id set.
 */
export function selectWindowScopeSnapshot(snapshot: WindowManagerSnapshot) {
  return { workspaceId: snapshot.workspaceId, windowIds: Object.keys(snapshot.windows) };
}

/**
 * Keep persisted per-window state — sessions-rail presentation and worktree
 * picks — aligned with live windows. `query` must be the authoritative
 * snapshot read for exactly this workspace and profile: it lists only that
 * partition's windows, so pruning is limited to the entries it owns. Only a
 * snapshot fetched (or streamed) since this read mounted or switched
 * partition counts: the cached one never goes stale, so returning to a
 * destination would otherwise prune against an old window list and drop
 * state another tab saved for a window created meanwhile. Before a fresh
 * snapshot lands the window set is unknown.
 */
export function useWindowScopePruning(
  profileId: string,
  query: Pick<
    UseQueryResult<ReturnType<typeof selectWindowScopeSnapshot>>,
    "data" | "isFetchedAfterMount"
  >
) {
  const snapshot = query.isFetchedAfterMount ? query.data : undefined;
  const partition = snapshot === undefined ? null : `${snapshot.workspaceId}\u0000${profileId}`;
  const liveWindowIds = snapshot?.windowIds.join(" ") ?? "";

  useEffect(() => {
    if (partition === null) return;
    const live = liveWindowIds === "" ? [] : liveWindowIds.split(" ");
    pruneSessionSidebarScopes(partition, live);
    pruneWorktreeScopes(partition, live);
  }, [partition, liveWindowIds]);
}
