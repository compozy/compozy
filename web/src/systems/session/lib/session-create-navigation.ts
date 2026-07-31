import type { SessionPayload } from "../types";

import { setActiveWorkspaceId } from "@/systems/workspace";

/**
 * The session route loads from the active workspace. Select the session owner before route
 * navigation so the canonical route can load and delegate window focus to the OS coordinator.
 */
export function activateCreatedSessionWorkspace(session: SessionPayload): void {
  const workspaceId = session.workspace_id?.trim();
  if (!workspaceId) {
    throw new Error("Created session requires a non-empty workspace_id");
  }
  setActiveWorkspaceId(workspaceId);
}
