import { createContext, useContext } from "react";

import { SHELL_WORKTREE_SCOPE } from "@/systems/workspace";

/**
 * The scope id whose per-window selection governs this subtree: inside a
 * retained OS window it is that window's id. Shared shell chrome reads the
 * effective selection scope; per-window controls and worktree-dialog writes use
 * the exact focused window's id. Other chrome writes read focus at action time.
 * It is `null` outside the shell entirely.
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
