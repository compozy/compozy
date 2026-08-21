import {
  selectWorktreeForScope,
  setActiveWorkspaceId,
  type WorkspaceScopeMode,
} from "@/systems/workspace";

export function applyPaletteWorktreeSelection(input: {
  readonly scope: WorkspaceScopeMode;
  readonly worktreeScopeId: string;
  readonly activeWorkspaceId: string | null;
  readonly entry: {
    readonly workspaceId?: string;
    readonly worktree?: { readonly id: string } | null;
  };
}): void {
  const targetWorkspaceId = input.entry.workspaceId ?? input.activeWorkspaceId;
  if (!input.entry.worktree || targetWorkspaceId === null) return;
  if (input.scope === "global") {
    setActiveWorkspaceId(targetWorkspaceId);
  }
  selectWorktreeForScope(input.worktreeScopeId, targetWorkspaceId, input.entry.worktree.id);
}
