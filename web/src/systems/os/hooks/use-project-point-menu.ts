import { useState } from "react";

import type { WorkspacePayload, WorkspaceProjectMenuPoint } from "@/systems/workspace";

/**
 * The project menu a workspace-menu row relays. It stays mounted after the
 * menu closes so the confirm dialog it opened survives, and it reads the
 * project's current payload while the catalog still lists it. Once the
 * project leaves the catalog it keeps the last payload only so a running
 * mutation's confirm can settle.
 */
export function useProjectPointMenu(workspaces: readonly WorkspacePayload[]) {
  const [target, setTarget] = useState<{
    workspace: WorkspacePayload;
    point: WorkspaceProjectMenuPoint | null;
  } | null>(null);
  const live = target
    ? workspaces.find(candidate => candidate.id === target.workspace.id)
    : undefined;
  return {
    workspace: live ?? target?.workspace ?? null,
    /** Whether the catalog still lists the project; an unlisted one offers no actions. */
    listed: live !== undefined,
    point: target?.point ?? null,
    open: (next: WorkspacePayload, point: WorkspaceProjectMenuPoint) =>
      setTarget({ workspace: next, point }),
    close: () => setTarget(current => (current ? { ...current, point: null } : null)),
  };
}
