import type { SessionPayload } from "../types";

import { setActiveWorkspaceId } from "@/systems/workspace";

/**
 * The session route loads from the active workspace. Select the session owner before route
 * navigation so the canonical route can load and delegate window focus to the OS coordinator.
 * Re-selecting the effective workspace would manufacture a switch barrier with no rebind to
 * complete it, so the current owner is deliberately a no-op.
 */
export function activateCreatedSessionWorkspace(
  session: SessionPayload,
  currentWorkspaceId: string | null,
  options: { skip?: boolean } = {}
): void {
  if (options.skip) return;
  const workspaceId = session.workspace_id?.trim();
  if (!workspaceId) {
    throw new Error("Created session requires a non-empty workspace_id");
  }
  if (workspaceId === currentWorkspaceId?.trim()) return;
  setActiveWorkspaceId(workspaceId);
}
