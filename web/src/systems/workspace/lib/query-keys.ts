import { computerQueryKey, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";

export function workspaceKeysFor(computerId: ComputerId = LOCAL_COMPUTER_ID) {
  const all = computerQueryKey(computerId, "workspaces");
  const lists = () => [...all, "list"] as const;
  const details = () => [...all, "detail"] as const;
  const allWorktrees = () => [...all, "worktrees"] as const;
  const worktrees = (workspaceID: string) => [...allWorktrees(), workspaceID] as const;
  const worktreeAggregates = () => [...allWorktrees(), "@aggregate"] as const;
  const worktreeDetail = (workspaceID: string, worktreeID: string) =>
    [...worktrees(workspaceID), worktreeID] as const;

  return {
    all,
    lists,
    list: () => [...lists()] as const,
    archived: () => [...lists(), "archived"] as const,
    details,
    detail: (workspaceID: string) => [...details(), workspaceID] as const,
    // Worktree reads are workspace-qualified so a catalog frame can only ever
    // invalidate the workspace it names.
    allWorktrees,
    worktrees,
    /** The All projects store listing; `@`-prefixed so no workspace id can collide. */
    worktreeAggregates,
    worktreeAggregate: (selection: readonly string[]) =>
      [...worktreeAggregates(), [...selection].sort().join(",")] as const,
    worktreeDetail,
    worktreeStatus: (workspaceID: string, worktreeID: string) =>
      [...worktreeDetail(workspaceID, worktreeID), "status"] as const,
    worktreeExit: (workspaceID: string, worktreeID: string) =>
      [...worktreeDetail(workspaceID, worktreeID), "exit"] as const,
    worktreeMaterializationFailure: (workspaceID: string, worktreeID: string) =>
      [...worktreeDetail(workspaceID, worktreeID), "materialization-failure"] as const,
  };
}

export const workspaceKeys = {
  ...workspaceKeysFor(LOCAL_COMPUTER_ID),
  forComputer: workspaceKeysFor,
};
