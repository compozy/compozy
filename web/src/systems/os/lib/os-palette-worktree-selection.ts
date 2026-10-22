import { selectWorktreeForScope, setActiveWorkspaceId } from "@/systems/workspace";

/**
 * A worktree row outside the focused project (All projects, User, or another
 * project) focuses its project before the per-window worktree is written.
 */
export function applyPaletteWorktreeSelection(input: {
  readonly worktreeScopeId: string;
  /** The focused project; `null` under All projects and User. */
  readonly activeWorkspaceId: string | null;
  readonly setActiveWorkspaceId?: (workspaceId: string) => void;
  readonly entry: {
    readonly workspaceId?: string;
    readonly worktree?: { readonly id: string } | null;
  };
}): void {
  const targetWorkspaceId = input.entry.workspaceId ?? input.activeWorkspaceId;
  if (!input.entry.worktree || targetWorkspaceId === null) return;
  if (targetWorkspaceId !== input.activeWorkspaceId) {
    (input.setActiveWorkspaceId ?? setActiveWorkspaceId)(targetWorkspaceId);
  }
  selectWorktreeForScope(input.worktreeScopeId, targetWorkspaceId, input.entry.worktree.id);
}
