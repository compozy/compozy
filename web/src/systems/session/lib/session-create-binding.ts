import type { WorkspaceScopeMode } from "@/systems/workspace";

export type SessionCreateBinding = { workspace: string } | { workspace_path: string };

/**
 * Global sessions bind to the hidden operator-home registration. No new API
 * field — `workspace` xor `workspace_path`.
 */
export function sessionCreateBinding(input: {
  scope: WorkspaceScopeMode;
  projectWorkspaceId: string | null;
  homeWorkspaceId: string | undefined;
  userHomeDir: string | undefined;
}): SessionCreateBinding | null {
  if (input.scope === "workspace") {
    return input.projectWorkspaceId ? { workspace: input.projectWorkspaceId } : null;
  }
  if (input.homeWorkspaceId) {
    return { workspace: input.homeWorkspaceId };
  }
  if (input.userHomeDir && input.userHomeDir.length > 0) {
    return { workspace_path: input.userHomeDir };
  }
  return null;
}
