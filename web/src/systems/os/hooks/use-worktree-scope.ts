import {
  effectiveWorktreeScopeId,
  SHELL_WORKTREE_SCOPE,
  useActiveWorktree,
  useWorktreeSelectionsByScope,
  type WorktreesResponse,
} from "@/systems/workspace";

import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

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
 * The scope whose stored selection the focused window reads, selected by value: the focused
 * window's own entry, else the shell's (`effectiveWorktreeScopeId`). It resolves the same
 * selection as `useFocusedWorktreeScopeId`, but focus moving between windows that read the same
 * entry (every window that never chose a worktree) changes nothing, so shell chrome reading it
 * does not re-render on a tab switch. Writes take `useFocusedWorktreeScopeReader` instead.
 */
export function useFocusedWorktreeReadScopeId(): string {
  const byScope = useWorktreeSelectionsByScope();
  return useDesktop(state =>
    effectiveWorktreeScopeId(byScope, state.focusedId ?? SHELL_WORKTREE_SCOPE)
  );
}

/**
 * The scope a shell gesture writes (a worktree or workspace picked from the menubar), read when
 * the gesture runs: the focused window's, else the shell's.
 */
export function useFocusedWorktreeScopeReader(): () => string {
  const { manager } = useOsShell();
  return () => manager.getState().focusedId ?? SHELL_WORKTREE_SCOPE;
}

/**
 * Resolve the shell worktree selection. Closed windows are pruned per
 * partition by `useWindowScopePruning`, never against the projection alone.
 */
export function useDesktopWorktreeScope(listing: WorktreesResponse | undefined) {
  const readScopeId = useFocusedWorktreeReadScopeId();
  const worktreeSelection = useActiveWorktree(readScopeId, listing);
  const worktreeScopeOf = useFocusedWorktreeScopeReader();
  return { readScopeId, worktreeScopeOf, worktreeSelection };
}
