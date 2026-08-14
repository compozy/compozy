import { useSelector } from "@xstate/store-react";

import { toWorktreeDisplayState } from "../lib/worktree-display";
import {
  activeWorkspaceSelectors,
  clearActiveWorktreeSelection,
  resolveScopedWorktreeId,
  setActiveWorktreeId,
  SHELL_WORKTREE_SCOPE,
} from "../stores/active-workspace-store";
import type { WorktreePayload, WorktreesResponse } from "../types";
import { useWorktrees } from "./use-worktrees";

export { SHELL_WORKTREE_SCOPE } from "../stores/active-workspace-store";

/** Why a selection stopped resolving. Both revert the scope to the parent. */
export type WorktreeFallbackReason = "missing" | "unavailable";

export interface ActiveWorktreeSelection {
  /** The selection as stored, even when it no longer resolves. */
  selectedWorktreeId: string | null;
  /** Resolved only when the worktree exists and can receive work. */
  activeWorktree: WorktreePayload | null;
  /** False until a stored selection has been checked against the live list. */
  resolved: boolean;
  /**
   * Set when a stored selection cannot receive work. The selection is kept, not
   * cleared: if the directory comes back, the scope resumes on its own.
   */
  fallback: { worktreeId: string; name: string | null; reason: WorktreeFallbackReason } | null;
}

export function useSelectedWorktreeId(scopeId: string): string | null {
  const byScope = useSelector(activeWorkspaceSelectors.worktreeByScope);
  return resolveScopedWorktreeId(byScope, scopeId);
}

/**
 * Resolves a scope's worktree selection against the live list at the read
 * boundary — never by syncing store state from an effect. A worktree that went
 * missing therefore degrades to the parent workspace plus a notice on the very
 * next render, with no write and no race.
 */
export function useActiveWorktree(
  scopeId: string,
  listing: WorktreesResponse | undefined
): ActiveWorktreeSelection {
  const selectedWorktreeId = useSelectedWorktreeId(scopeId);
  if (selectedWorktreeId === null) {
    return { selectedWorktreeId: null, activeWorktree: null, resolved: true, fallback: null };
  }

  if (listing === undefined) {
    return { selectedWorktreeId, activeWorktree: null, resolved: false, fallback: null };
  }

  const match = listing.worktrees.find(worktree => worktree.id === selectedWorktreeId);
  if (!match) {
    return {
      selectedWorktreeId,
      activeWorktree: null,
      resolved: true,
      fallback: { worktreeId: selectedWorktreeId, name: null, reason: "missing" },
    };
  }

  const displayState = toWorktreeDisplayState(match);
  if (displayState !== "ready") {
    return {
      selectedWorktreeId,
      activeWorktree: null,
      resolved: true,
      fallback: {
        worktreeId: selectedWorktreeId,
        name: match.name,
        reason: displayState === "missing" ? "missing" : "unavailable",
      },
    };
  }

  return { selectedWorktreeId, activeWorktree: match, resolved: true, fallback: null };
}

export interface ScopedWorktreeFilter {
  worktreeId: string | undefined;
  /** True only after a stored selection is known to be ready or a valid fallback. */
  resolved: boolean;
}

/**
 * The worktree id a scoped list should filter by, or `undefined` when the scope
 * is the workspace root.
 *
 * A selection that went missing or is not ready resolves to `undefined`, so the
 * list falls back to the parent workspace exactly as the menubar notice says —
 * the view and the notice can never disagree.
 */
export function useScopedWorktreeFilter(
  workspaceId: string | null,
  scopeId: string,
  options: { enabled?: boolean } = {}
): ScopedWorktreeFilter {
  const enabled = (options.enabled ?? true) && workspaceId !== null;
  const worktrees = useWorktrees(workspaceId, { enabled });
  const selection = useActiveWorktree(scopeId, worktrees.data);
  if (!enabled) return { worktreeId: undefined, resolved: true };
  return { worktreeId: selection.activeWorktree?.id, resolved: selection.resolved };
}

export function selectWorktreeForScope(
  scopeId: string,
  workspaceId: string,
  worktreeId: string
): void {
  setActiveWorktreeId(scopeId, workspaceId, worktreeId);
}

export function clearWorktreeForScope(scopeId: string = SHELL_WORKTREE_SCOPE): void {
  clearActiveWorktreeSelection(scopeId);
}
