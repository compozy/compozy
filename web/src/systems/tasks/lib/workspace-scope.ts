export type ActiveTaskScopeFilter =
  | { scope: "global"; workspace?: undefined }
  | { scope: "workspace"; workspace: string };

export function taskScopeForActiveWorkspace(
  scope: "global" | "workspace" | null | undefined,
  workspaceId?: string | null
): ActiveTaskScopeFilter | null {
  if (scope === "global") {
    return { scope: "global" };
  }
  if (scope === "workspace" && workspaceId) {
    return { scope: "workspace", workspace: workspaceId };
  }
  return null;
}
