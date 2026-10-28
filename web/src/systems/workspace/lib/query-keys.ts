export const workspaceKeys = {
  all: ["workspaces"] as const,
  lists: () => [...workspaceKeys.all, "list"] as const,
  list: () => [...workspaceKeys.lists()] as const,
  archived: () => [...workspaceKeys.lists(), "archived"] as const,
  details: () => [...workspaceKeys.all, "detail"] as const,
  detail: (workspaceID: string) => [...workspaceKeys.details(), workspaceID] as const,
  // Worktree reads are workspace-qualified so a catalog frame can only ever
  // invalidate the workspace it names.
  allWorktrees: () => [...workspaceKeys.all, "worktrees"] as const,
  worktrees: (workspaceID: string) => [...workspaceKeys.allWorktrees(), workspaceID] as const,
  /** The All projects store listing; `@`-prefixed so no workspace id can collide. */
  worktreeAggregates: () => [...workspaceKeys.allWorktrees(), "@aggregate"] as const,
  worktreeAggregate: (selection: readonly string[]) =>
    [...workspaceKeys.worktreeAggregates(), [...selection].sort().join(",")] as const,
  worktreeDetail: (workspaceID: string, worktreeID: string) =>
    [...workspaceKeys.worktrees(workspaceID), worktreeID] as const,
  worktreeStatus: (workspaceID: string, worktreeID: string) =>
    [...workspaceKeys.worktreeDetail(workspaceID, worktreeID), "status"] as const,
  worktreeExit: (workspaceID: string, worktreeID: string) =>
    [...workspaceKeys.worktreeDetail(workspaceID, worktreeID), "exit"] as const,
  worktreeMaterializationFailure: (workspaceID: string, worktreeID: string) =>
    [...workspaceKeys.worktreeDetail(workspaceID, worktreeID), "materialization-failure"] as const,
};
