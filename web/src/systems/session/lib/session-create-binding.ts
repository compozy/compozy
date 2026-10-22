import type { DestinationKind } from "@/systems/workspace";

export type SessionCreateBinding = { workspace: string } | { scope: "user" };

/** The request fields that start a session in User: no workspace, no path, no worktree. */
export function userSessionBinding(): SessionCreateBinding {
  return { scope: "user" };
}

/**
 * A project binds to its workspace; without the id the submit withholds.
 * Outside a project the session starts in User (`scope: "user"`).
 */
export function sessionCreateBinding(input: {
  scope: DestinationKind;
  projectWorkspaceId: string | null;
}): SessionCreateBinding | null {
  if (input.scope === "project") {
    return input.projectWorkspaceId ? { workspace: input.projectWorkspaceId } : null;
  }
  return userSessionBinding();
}
