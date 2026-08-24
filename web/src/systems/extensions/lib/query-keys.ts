/**
 * The daemon keys extension instances by `(name, workspace, profile)`: a workspace dev overlay and
 * the global published row are different rows, and profile placement changes the inventory answer.
 * Every cached list therefore carries both normalized identity axes.
 */
export const EXTENSION_GLOBAL_WORKSPACE_KEY = "__global__";

export function extensionWorkspaceKey(workspaceId?: string | null): string {
  const normalized = typeof workspaceId === "string" ? workspaceId.trim() : "";
  return normalized === "" ? EXTENSION_GLOBAL_WORKSPACE_KEY : normalized;
}

export function extensionProfileKey(profileName?: string | null): string {
  const normalized = typeof profileName === "string" ? profileName.trim() : "";
  return normalized === "" ? "default" : normalized;
}

export const extensionKeys = {
  all: ["extensions"] as const,
  lists: () => [...extensionKeys.all, "list"] as const,
  list: (workspaceId?: string | null, profileName?: string | null) =>
    [
      ...extensionKeys.lists(),
      extensionWorkspaceKey(workspaceId),
      extensionProfileKey(profileName),
    ] as const,
  provenance: (name: string) => [...extensionKeys.all, "provenance", name] as const,
  logs: (name: string, workspaceId?: string | null) =>
    [...extensionKeys.all, "logs", extensionWorkspaceKey(workspaceId), name.trim()] as const,
  /**
   * The inventory route carries no workspace selector and resolves the global published instance,
   * so the key stays name-only rather than implying a scope the route does not expose.
   */
  inventory: (name: string) => [...extensionKeys.all, "inventory", name] as const,
};
