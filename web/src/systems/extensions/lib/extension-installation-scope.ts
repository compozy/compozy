import type { ExtensionEntry, ExtensionInstanceScope, ExtensionUpdateRequest } from "../types";

/** Mutations target the selected installation, including globals inherited by a workspace. */
export function extensionInstallationScope(extension: ExtensionEntry): ExtensionInstanceScope {
  return {
    profileName: extension.profile,
    ...(extension.workspace_id ? { workspaceId: extension.workspace_id } : {}),
  };
}

export function extensionUpdateScope(
  extension: ExtensionEntry
): Pick<ExtensionUpdateRequest, "scope" | "profile" | "workspace_id"> {
  return {
    profile: extension.profile,
    scope: extension.workspace_id ? "workspace" : "global",
    ...(extension.workspace_id ? { workspace_id: extension.workspace_id } : {}),
  };
}
