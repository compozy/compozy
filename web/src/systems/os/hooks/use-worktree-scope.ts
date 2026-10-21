import {
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

/**
 * Resolve the shell worktree selection. Closed windows are pruned per
 * partition by `useWindowScopePruning`, never against the projection alone.
 */
export function useDesktopWorktreeScope(listing: WorktreesResponse | undefined) {
  const worktreeScopeId = useFocusedWorktreeScopeId();
  const worktreeSelection = useActiveWorktree(worktreeScopeId, listing);
  return { worktreeScopeId, worktreeSelection };
}
