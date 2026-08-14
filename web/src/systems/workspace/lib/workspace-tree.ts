import type { WorktreesResponse } from "../types";
import { partitionProjectWorkspaces, type WorkspaceHomeCandidate } from "./project-workspaces";
import { toWorktreeNestEntries, type WorktreeNestEntry } from "./worktree-display";
import { adoptedWorktreeCount, runningAgentCount, sortWorktreeNestEntries } from "./worktree-sort";

/**
 * A workspace plus its worktrees. Nesting depth is fixed at 2 by construction:
 * this node has no child-workspace slot, so no surface can render a worktree
 * under a worktree (US-010 EC-1) without changing the type.
 */
export interface WorkspaceTreeNode<T extends WorkspaceHomeCandidate> {
  workspace: T;
  /** `false` for a non-git workspace: every worktree affordance is absent. */
  gitBacked: boolean;
  /** Locked sort order, already applied. */
  worktrees: WorktreeNestEntry[];
  /** Adopted-only, per the locked count rule. */
  adoptedCount: number;
  /** Collapsed-parent aggregate; `0` renders nothing. */
  runningAgents: number;
}

export type WorktreeListingByWorkspace = Readonly<
  Record<string, Pick<WorktreesResponse, "worktrees" | "discovered" | "repo"> | undefined>
>;

/**
 * Single projection behind the switcher, the menubar menu, and the overview, so
 * the three surfaces cannot diverge. The operator-home registration is hidden;
 * Global is a scope, not a project workspace row.
 */
export function groupWorkspaceTree<T extends WorkspaceHomeCandidate & { id: string }>(
  workspaces: ReadonlyArray<T> | undefined,
  listings: WorktreeListingByWorkspace,
  userHomeDir: string | undefined
): WorkspaceTreeNode<T>[] {
  const { projectWorkspaces } = partitionProjectWorkspaces(workspaces, userHomeDir);

  return projectWorkspaces.map(workspace => {
    const listing = listings[workspace.id];
    const gitBacked = listing?.repo.git_backed ?? false;
    // A non-git workspace has no worktree group at all — not an empty one.
    const worktrees = gitBacked ? sortWorktreeNestEntries(toWorktreeNestEntries(listing)) : [];

    return {
      workspace,
      gitBacked,
      worktrees,
      adoptedCount: adoptedWorktreeCount(worktrees),
      runningAgents: runningAgentCount(worktrees),
    };
  });
}
