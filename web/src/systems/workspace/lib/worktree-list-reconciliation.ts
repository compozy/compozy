import type { WorktreePayload, WorktreesResponse } from "../types";

/**
 * Upserts a mutation result into the cached list envelope, preserving row order
 * so a create/adopt never reshuffles the nest under the user's cursor. A newly
 * adopted worktree also leaves the `discovered` array, because it now has a
 * record and must stop rendering as an adoption candidate.
 */
export function reconcileWorktreeList(
  current: WorktreesResponse | undefined,
  worktree: WorktreePayload
): WorktreesResponse | undefined {
  if (!current) return current;

  const index = current.worktrees.findIndex(item => item.id === worktree.id);
  const worktrees =
    index === -1
      ? [...current.worktrees, worktree]
      : current.worktrees.map(item => (item.id === worktree.id ? worktree : item));

  return {
    ...current,
    worktrees,
    discovered: current.discovered.filter(item => item.path !== worktree.path),
  };
}

/** Drops a removed or dismissed record. Discovered entries are untouched. */
export function removeWorktreeFromList(
  current: WorktreesResponse | undefined,
  worktreeID: string
): WorktreesResponse | undefined {
  if (!current) return current;

  return {
    ...current,
    worktrees: current.worktrees.filter(item => item.id !== worktreeID),
  };
}
