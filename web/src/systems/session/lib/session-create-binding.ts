import type { DestinationKind } from "@/systems/workspace";

export type SessionCreateBinding = { workspace: string } | { workspace_path: string };

/**
 * A project binds to its workspace; without the id the submit withholds.
 * Outside a project the request keeps the operator-home path until User
 * sessions get their own create scope. No new API field — `workspace` xor
 * `workspace_path`.
 */
export function sessionCreateBinding(input: {
  scope: DestinationKind;
  projectWorkspaceId: string | null;
  userHomeDir: string | undefined;
}): SessionCreateBinding | null {
  if (input.scope === "project") {
    return input.projectWorkspaceId ? { workspace: input.projectWorkspaceId } : null;
  }
  if (input.userHomeDir && input.userHomeDir.length > 0) {
    return { workspace_path: input.userHomeDir };
  }
  return null;
}
