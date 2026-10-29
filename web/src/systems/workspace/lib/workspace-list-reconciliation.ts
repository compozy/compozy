import type { WorkspacePayload } from "../types";

/** Reconciles a create-or-resolve response into the ordered workspace registry. */
export function reconcileWorkspaceList(
  current: WorkspacePayload[] | undefined,
  workspace: WorkspacePayload
): WorkspacePayload[] {
  const others = (current ?? []).filter(item => item.id !== workspace.id);
  return workspace.archived_at ? others : [workspace, ...others];
}
