/**
 * The daemon keys extension instances by `(name, workspace)`: a workspace dev overlay and the
 * global published row are different rows behind the same name. Every cached read therefore
 * carries the normalized workspace identity so a projection is never reused across workspaces.
 */
export const EXTENSION_GLOBAL_WORKSPACE_KEY = "__global__";

export function extensionWorkspaceKey(workspaceId?: string | null): string {
  const normalized = typeof workspaceId === "string" ? workspaceId.trim() : "";
  return normalized === "" ? EXTENSION_GLOBAL_WORKSPACE_KEY : normalized;
}

export const extensionKeys = {
  all: ["extensions"] as const,
  lists: () => [...extensionKeys.all, "list"] as const,
  list: (workspaceId?: string | null) =>
    [...extensionKeys.lists(), extensionWorkspaceKey(workspaceId)] as const,
  provenance: (name: string) => [...extensionKeys.all, "provenance", name] as const,
  logs: (name: string, workspaceId?: string | null) =>
    [...extensionKeys.all, "logs", extensionWorkspaceKey(workspaceId), name.trim()] as const,
  /**
   * The inventory route carries no workspace selector and resolves the global published instance,
   * so the key stays name-only rather than implying a scope the route does not expose.
   */
  inventory: (name: string) => [...extensionKeys.all, "inventory", name] as const,
};
