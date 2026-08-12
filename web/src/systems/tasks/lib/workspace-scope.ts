/**
 * The daemon rejects a worktree filter without a workspace, so the union makes
 * that unrepresentable: only the workspace branch can carry a worktree.
 */
export type ActiveTaskScopeFilter =
  | { scope: "global"; workspace?: undefined; worktree?: undefined }
  | { scope: "workspace"; workspace: string; worktree?: string };

export function taskScopeForActiveWorkspace(
  scope: "global" | "workspace" | null | undefined,
  workspaceId?: string | null,
  activeWorktreeId?: string | null
): ActiveTaskScopeFilter | null {
  if (scope === "global") {
    return { scope: "global" };
  }
  if (scope === "workspace") {
    if (!workspaceId) return null;
    const worktree = activeWorktreeId?.trim();
    return {
      scope: "workspace",
      workspace: workspaceId,
      ...(worktree ? { worktree } : {}),
    };
  }
  return null;
}
