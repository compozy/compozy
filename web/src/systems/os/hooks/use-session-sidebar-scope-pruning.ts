import { useEffect } from "react";

import { pruneSessionSidebarScopes } from "@/systems/session";

import type { WindowManagerSnapshot } from "../lib/window-manager-types";

/**
 * Keep persisted per-window sessions-rail state aligned with live windows.
 * `snapshot` must be the authoritative read for exactly this workspace and
 * profile: it lists only that partition's windows, so pruning is limited to the
 * entries it owns. Before the first snapshot lands the window set is unknown,
 * and pruning against it would drop every window's rail state.
 */
export function useSessionSidebarScopePruning(
  profileId: string,
  snapshot: WindowManagerSnapshot | undefined
) {
  const partition = snapshot === undefined ? null : `${snapshot.workspaceId}\u0000${profileId}`;
  const liveWindowIds = snapshot === undefined ? "" : Object.keys(snapshot.windows).join(" ");

  useEffect(() => {
    if (partition === null) return;
    pruneSessionSidebarScopes(partition, liveWindowIds === "" ? [] : liveWindowIds.split(" "));
  }, [partition, liveWindowIds]);
}
