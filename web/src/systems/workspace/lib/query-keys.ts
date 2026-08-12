export const workspaceKeys = {
  all: ["workspaces"] as const,
  lists: () => [...workspaceKeys.all, "list"] as const,
  list: () => [...workspaceKeys.lists()] as const,
  details: () => [...workspaceKeys.all, "detail"] as const,
  detail: (workspaceID: string) => [...workspaceKeys.details(), workspaceID] as const,
  // Worktree reads are workspace-qualified so a catalog frame can only ever
  // invalidate the workspace it names.
  allWorktrees: () => [...workspaceKeys.all, "worktrees"] as const,
  worktrees: (workspaceID: string) => [...workspaceKeys.allWorktrees(), workspaceID] as const,
  worktreeDetail: (workspaceID: string, worktreeID: string) =>
    [...workspaceKeys.worktrees(workspaceID), worktreeID] as const,
};
