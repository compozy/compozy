import type { WorkspacePayload } from "../types";

/** Reconciles a create-or-resolve response into the ordered workspace registry. */
export function reconcileWorkspaceList(
  current: WorkspacePayload[] | undefined,
  workspace: WorkspacePayload
): WorkspacePayload[] {
  return [workspace, ...(current ?? []).filter(item => item.id !== workspace.id)];
}
