import type { WorkspacePayload } from "../types";

export function selectActiveWorkspace(
  workspaces: readonly WorkspacePayload[],
  selectedWorkspaceId: string | null
): WorkspacePayload | undefined {
  if (selectedWorkspaceId) {
    const selectedWorkspace = workspaces.find(workspace => workspace.id === selectedWorkspaceId);
    if (selectedWorkspace) {
      return selectedWorkspace;
    }
  }

  return workspaces[0];
}
