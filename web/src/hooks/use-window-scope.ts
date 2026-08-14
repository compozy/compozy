import { createContext, useContext } from "react";

import { SHELL_WORKTREE_SCOPE } from "@/systems/workspace";

/**
 * The scope id whose per-window selection governs this subtree: inside a
 * retained OS window it is that window's id; in desktop-shell chrome it is the
 * focused window's scope (provided at the shell root, so chrome gestures act on
 * the same scope the menubar chip reports); `null` outside the shell entirely.
 *
 * It lives in shared hooks rather than the `os` system so domain systems can
 * read their own window scope without importing `os` — `os` already depends on
 * several of them, and the reverse edge would be a cycle.
 */
export const WindowScopeContext = createContext<string | null>(null);

/**
 * The scope owning per-window selection here: this window, or the shell scope
 * for surfaces rendered outside a window.
 */
export function useWorktreeScopeId(): string {
  return useContext(WindowScopeContext) ?? SHELL_WORKTREE_SCOPE;
}
