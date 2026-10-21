import type { DataScope } from "@/systems/workspace";

/**
 * The daemon rejects a worktree filter without a workspace, so the union makes
 * that unrepresentable: only the workspace branch can carry a worktree.
 */
export type ActiveTaskScopeFilter =
  | { scope: "global"; workspace?: undefined; worktree?: undefined }
  | { scope: "workspace"; workspace: string; worktree?: string };

/**
 * Task listing scope for a data scope. User tasks are `scope=global` (no
 * workspace binding); All projects keeps that population until it gains its
 * own aggregate. `null` while the destination is unsettled.
 */
export function taskScopeForActiveWorkspace(
  dataScope: DataScope | null | undefined,
  activeWorktreeId?: string | null
): ActiveTaskScopeFilter | null {
  if (!dataScope) return null;
  if (dataScope.kind !== "project") {
    return { scope: "global" };
  }
  const worktree = activeWorktreeId?.trim();
  return {
    scope: "workspace",
    workspace: dataScope.workspaceId,
    ...(worktree ? { worktree } : {}),
  };
}
