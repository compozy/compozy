export const ACTIVE_WORKSPACE_HEADER = "X-Compozy-Workspace-ID";

export function workspaceHeaders(workspaceId: string): Record<string, string> {
  return { [ACTIVE_WORKSPACE_HEADER]: workspaceId };
}
