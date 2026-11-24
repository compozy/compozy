import { computerQueryKey, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";

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
  all: computerQueryKey(LOCAL_COMPUTER_ID, "extensions"),
  lists: () => [...extensionKeys.all, "list"] as const,
  list: (workspaceId?: string | null, profileName?: string | null) =>
    [
      ...extensionKeys.lists(),
      extensionWorkspaceKey(workspaceId),
      extensionProfileKey(profileName),
    ] as const,
  provenance: (name: string) => [...extensionKeys.all, "provenance", name] as const,
  /** Tombstones are global: bundled rows are global, so the list carries no workspace or profile. */
  removedBuiltins: () => [...extensionKeys.all, "removed-builtins"] as const,
  logs: (name: string, workspaceId?: string | null, profileName?: string | null) =>
    [
      ...extensionKeys.all,
      "logs",
      extensionWorkspaceKey(workspaceId),
      extensionProfileKey(profileName),
      name.trim(),
    ] as const,
  inventory: (name: string, workspaceId?: string | null, profileName?: string | null) =>
    [
      ...extensionKeys.all,
      "inventory",
      name,
      extensionWorkspaceKey(workspaceId),
      extensionProfileKey(profileName),
    ] as const,
};
