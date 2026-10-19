import { useEffect } from "react";

import { pruneSessionSidebarScopes } from "@/systems/session";
import {
  pruneWorktreeScopes,
  SHELL_WORKTREE_SCOPE,
  useActiveWorktree,
  type WorktreesResponse,
} from "@/systems/workspace";

import { useDesktop } from "./use-desktop";

export { useWorktreeScopeId, WindowScopeContext } from "@/hooks/use-window-scope";

/**
 * The scope shell chrome reads and writes. The menubar chip must say where new
 * work will run, and with per-window scopes that is the focused window — so the
 * chip follows focus and switching from the menubar retargets that same window.
 */
export function useFocusedWorktreeScopeId(): string {
  const focusedId = useDesktop(state => state.focusedId);
  return focusedId ?? SHELL_WORKTREE_SCOPE;
}

/** Resolve shell worktree selection and keep persisted scopes aligned with live windows. */
export function useDesktopWorktreeScope(listing: WorktreesResponse | undefined) {
  const worktreeScopeId = useFocusedWorktreeScopeId();
  const worktreeSelection = useActiveWorktree(worktreeScopeId, listing);
  const liveWindowIds = useDesktop(state => Object.keys(state.windows).join(" "));

  useEffect(() => {
    pruneWorktreeScopes(liveWindowIds === "" ? [] : liveWindowIds.split(" "));
  }, [liveWindowIds]);

  return { worktreeScopeId, worktreeSelection };
}

/**
 * Keep persisted per-window sessions-rail state aligned with live windows. The
 * window set is unknown until the first daemon snapshot lands; pruning against
 * that empty pre-hydration projection would drop every window's rail state.
 */
export function useSessionSidebarScopePruning() {
  const liveWindowIds = useDesktop(state =>
    state.snapshot === null ? null : Object.keys(state.snapshot.windows).join(" ")
  );

  useEffect(() => {
    if (liveWindowIds === null) return;
    pruneSessionSidebarScopes(liveWindowIds === "" ? [] : liveWindowIds.split(" "));
  }, [liveWindowIds]);
}
